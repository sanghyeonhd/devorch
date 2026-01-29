package okaon

import "context"

// Store는 OkAON 데이터 저장소 인터페이스
type Store interface {
	// 기본 데이터 삽입
	InsertWork(ctx context.Context, w Work) error
	InsertRun(ctx context.Context, r Run) error
	InsertQuality(ctx context.Context, q Quality) error
	InsertReward(ctx context.Context, r Reward) error

	// Arm 통계 조회/업데이트
	UpsertArmStats(ctx context.Context, u ArmStatUpdate) error
	GetArmStats(ctx context.Context, fingerprint, category, provider, model string) (ArmStat, bool, error)

	// 집계 쿼리
	GetModelAggregate(ctx context.Context, since int64) ([]ModelAggregate, error)
	GetCategoryAggregate(ctx context.Context, since int64) ([]CategoryAggregate, error)
}
