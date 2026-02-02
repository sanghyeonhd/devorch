package learning

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// SelectBestAgent selects the best agent for a given task type using bandit algorithm
func (l *Learner) SelectBestAgent(ctx context.Context, taskType string, agents []string) (string, error) {
	if len(agents) == 0 {
		return "", fmt.Errorf("no agents available")
	}

	if len(agents) == 1 {
		return agents[0], nil
	}

	// Convert agent names to bandit arms
	var arms []bandit.ArmKey
	for _, agent := range agents {
		armKey := bandit.ArmKey(fmt.Sprintf("%s:%s", taskType, agent))
		arms = append(arms, armKey)
	}

	// Use bandit algorithm to select best arm
	selectedArm, err := l.bandit.Select(arms)
	if err != nil {
		// Fallback to random selection
		return agents[0], nil
	}

	// Extract agent name from selected arm
	armStr := string(selectedArm)
	if colon := len(taskType) + 1; colon < len(armStr) {
		return armStr[colon:], nil
	}

	return agents[0], nil
}

// RecordAgentPerformance records the performance of an agent for learning
func (l *Learner) RecordAgentPerformance(ctx context.Context, agentName string, taskType string, success bool, duration time.Duration) error {
	// Convert success/failure to reward (0.0 = failure, 1.0 = success)
	reward := 0.0
	if success {
		reward = 1.0

		// Adjust reward based on speed (faster = better reward)
		// Max reward for tasks completed in under 1 second
		// Linear decrease for longer tasks, minimum 0.1
		speedBonus := 1.0
		if duration > time.Second {
			speedBonus = 1.0 / (float64(duration) / float64(time.Second))
			if speedBonus < 0.1 {
				speedBonus = 0.1
			}
		}
		reward = reward * speedBonus
	}

	// Create bandit arm key
	armKey := bandit.ArmKey(fmt.Sprintf("%s:%s", taskType, agentName))

	// Update bandit with observed reward
	return l.bandit.Update(armKey, reward)
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

	// Convert bandit stats back to OkAON format
	// Note: This is a simplified conversion for the learning demo
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
