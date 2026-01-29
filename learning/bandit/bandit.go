package bandit

// ArmKey = context fingerprint + model/provider 등 선택 단위
type ArmKey string

type ArmStats struct {
	Alpha      float64
	Beta       float64
	Pulls      int64
	LastReward float64
}

type Store interface {
	Load(arm ArmKey) (ArmStats, bool, error)
	Save(arm ArmKey, st ArmStats) error
}

// Bandit: 컨텍스트가 "해시/티어/카테고리" 정도로 들어오는 경량 구조로 시작
type Bandit interface {
	Select(arms []ArmKey) (ArmKey, error)
	Update(chosen ArmKey, reward float64) error
}
