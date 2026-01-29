package learning

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"devorch/internal/okaon"
	"devorch/learning/bandit"
)

type OkAONStore interface {
	GetArmStats(ctx context.Context, fingerprint, category, provider, model string) (okaon.ArmStat, bool, error)
	UpsertArmStats(ctx context.Context, u okaon.ArmStatUpdate) error
}

type Learner struct {
	okstore OkAONStore
	bandit  bandit.Bandit
}

func NewLearner(okstore OkAONStore, b bandit.Bandit) *Learner {
	return &Learner{okstore: okstore, bandit: b}
}

// ArmKey: contextHash + model => arm
func ArmKey(contextHash, model string) string {
	h := sha256.Sum256([]byte(contextHash + "::" + model))
	return hex.EncodeToString(h[:])
}

// Reward를 [0,1]로 정규화해서 bandit에 업데이트
func (l *Learner) ObserveReward(ctx context.Context, contextHash, model string, reward01 float64) error {
	key := ArmKey(contextHash, model)
	return l.bandit.Update(bandit.ArmKey(key), reward01)
}

// OkAON-backed bandit.Store 구현을 위해 adaptor 제공
type BanditStore struct {
	Ctx         context.Context
	OkStore     OkAONStore
	Fingerprint string
	Category    string
}

func (bs BanditStore) Load(arm bandit.ArmKey) (bandit.ArmStats, bool, error) {
	// ArmKey에서 provider:model 파싱 시도
	provider, model := parseArmKey(string(arm))
	st, ok, err := bs.OkStore.GetArmStats(bs.Ctx, bs.Fingerprint, bs.Category, provider, model)
	if err != nil {
		return bandit.ArmStats{}, false, err
	}
	if !ok {
		return bandit.ArmStats{}, false, nil
	}
	// ArmStats를 BanditStats로 변환 (mean/stddev -> alpha/beta 변환)
	alpha := st.MeanReward01*float64(st.CountRuns) + 1
	beta := (1-st.MeanReward01)*float64(st.CountRuns) + 1
	return bandit.ArmStats{Alpha: alpha, Beta: beta, Pulls: st.CountRuns, LastReward: st.MeanReward01}, true, nil
}

func (bs BanditStore) Save(arm bandit.ArmKey, st bandit.ArmStats) error {
	provider, model := parseArmKey(string(arm))
	return bs.OkStore.UpsertArmStats(bs.Ctx, okaon.ArmStatUpdate{
		Fingerprint:    bs.Fingerprint,
		Category:       bs.Category,
		Provider:       provider,
		Model:          model,
		DeltaCount:     1,
		DeltaSumReward: st.LastReward,
		DeltaSumSq:     st.LastReward * st.LastReward,
	})
}

func parseArmKey(key string) (provider, model string) {
	// 간단한 파싱: key가 provider:model 형태라고 가정
	// 실제로는 더 복잡할 수 있음
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == ':' {
			return key[:i], key[i+1:]
		}
	}
	return "", key
}

// UpdateArmStats updates arm statistics with a new reward observation
func (l *Learner) UpdateArmStats(ctx context.Context, fingerprint, category, provider, model string, reward01 float64) error {
	return l.okstore.UpsertArmStats(ctx, okaon.ArmStatUpdate{
		Fingerprint:    fingerprint,
		Category:       category,
		Provider:       provider,
		Model:          model,
		DeltaCount:     1,
		DeltaSumReward: reward01,
		DeltaSumSq:     reward01 * reward01,
	})
}

// GetArmStats retrieves arm statistics
func (l *Learner) GetArmStats(ctx context.Context, fingerprint, category, provider, model string) (okaon.ArmStat, bool, error) {
	return l.okstore.GetArmStats(ctx, fingerprint, category, provider, model)
}

// Dummy usage of time package to avoid import error
var _ = time.Now
