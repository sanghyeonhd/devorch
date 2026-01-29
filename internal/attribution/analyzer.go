package attribution

import (
	"context"
	"fmt"
	"sort"
)

// Cause는 성능 변화의 원인을 나타냅니다
type Cause struct {
	Dimension string // model | prompt | gpu_driver | runtime | backend
	OldValue  string
	NewValue  string
	Score     float64 // 0.0 ~ 1.0, 높을수록 해당 요인의 영향이 큼
}

// BenchmarkData는 분석에 사용되는 벤치마크 데이터입니다
type BenchmarkData struct {
	ID        string
	Model     string
	Provider  string
	Backend   string
	GPUDriver string
	LatencyMs int64
	Error     string
}

// Analyzer는 성능 드리프트의 원인을 분석합니다
type Analyzer struct {
	// 벤치마크 데이터를 조회하는 함수 (외부 주입)
	FetchBenchmarks func(ctx context.Context, ids []string) ([]BenchmarkData, error)
}

// NewAnalyzer는 새 Analyzer를 생성합니다
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// WithBenchmarkFetcher는 벤치마크 데이터 조회 함수를 설정합니다
func (a *Analyzer) WithBenchmarkFetcher(fn func(ctx context.Context, ids []string) ([]BenchmarkData, error)) *Analyzer {
	a.FetchBenchmarks = fn
	return a
}

// Analyze는 최근 벤치마크와 기준선 벤치마크를 비교하여 변화 원인을 분석합니다
func (a *Analyzer) Analyze(ctx context.Context, recentBenchIDs []string, baselineBenchIDs []string) ([]Cause, error) {
	if len(recentBenchIDs) == 0 || len(baselineBenchIDs) == 0 {
		return nil, fmt.Errorf("both recent and baseline benchmark IDs are required")
	}

	// 벤치마크 데이터가 주입되지 않은 경우 빈 결과 반환
	if a.FetchBenchmarks == nil {
		return []Cause{}, nil
	}

	// 데이터 조회
	recentData, err := a.FetchBenchmarks(ctx, recentBenchIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent benchmarks: %w", err)
	}
	baselineData, err := a.FetchBenchmarks(ctx, baselineBenchIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch baseline benchmarks: %w", err)
	}

	// 각 차원별 변화 분석
	causes := []Cause{}

	// 모델 변화 분석
	if cause := a.analyzeModelChange(recentData, baselineData); cause != nil {
		causes = append(causes, *cause)
	}

	// Provider 변화 분석
	if cause := a.analyzeProviderChange(recentData, baselineData); cause != nil {
		causes = append(causes, *cause)
	}

	// Backend 변화 분석
	if cause := a.analyzeBackendChange(recentData, baselineData); cause != nil {
		causes = append(causes, *cause)
	}

	// GPU Driver 변화 분석
	if cause := a.analyzeGPUDriverChange(recentData, baselineData); cause != nil {
		causes = append(causes, *cause)
	}

	// 점수순 정렬 (높은 점수가 먼저)
	sort.Slice(causes, func(i, j int) bool {
		return causes[i].Score > causes[j].Score
	})

	return causes, nil
}

// getMostFrequent는 문자열 슬라이스에서 가장 빈번한 값을 반환합니다
func getMostFrequent(values []string) string {
	if len(values) == 0 {
		return ""
	}
	counts := make(map[string]int)
	for _, v := range values {
		counts[v]++
	}
	var maxKey string
	maxCount := 0
	for k, c := range counts {
		if c > maxCount {
			maxCount = c
			maxKey = k
		}
	}
	return maxKey
}

func (a *Analyzer) analyzeModelChange(recent, baseline []BenchmarkData) *Cause {
	recentModels := make([]string, len(recent))
	for i, d := range recent {
		recentModels[i] = d.Model
	}
	baselineModels := make([]string, len(baseline))
	for i, d := range baseline {
		baselineModels[i] = d.Model
	}

	recentMost := getMostFrequent(recentModels)
	baselineMost := getMostFrequent(baselineModels)

	if recentMost != baselineMost && recentMost != "" && baselineMost != "" {
		// 모델 변경 빈도를 점수로 계산
		changeCount := 0
		for _, m := range recentModels {
			if m != baselineMost {
				changeCount++
			}
		}
		score := float64(changeCount) / float64(len(recentModels))

		return &Cause{
			Dimension: "model",
			OldValue:  baselineMost,
			NewValue:  recentMost,
			Score:     score,
		}
	}
	return nil
}

func (a *Analyzer) analyzeProviderChange(recent, baseline []BenchmarkData) *Cause {
	recentProviders := make([]string, len(recent))
	for i, d := range recent {
		recentProviders[i] = d.Provider
	}
	baselineProviders := make([]string, len(baseline))
	for i, d := range baseline {
		baselineProviders[i] = d.Provider
	}

	recentMost := getMostFrequent(recentProviders)
	baselineMost := getMostFrequent(baselineProviders)

	if recentMost != baselineMost && recentMost != "" && baselineMost != "" {
		changeCount := 0
		for _, p := range recentProviders {
			if p != baselineMost {
				changeCount++
			}
		}
		score := float64(changeCount) / float64(len(recentProviders))

		return &Cause{
			Dimension: "provider",
			OldValue:  baselineMost,
			NewValue:  recentMost,
			Score:     score,
		}
	}
	return nil
}

func (a *Analyzer) analyzeBackendChange(recent, baseline []BenchmarkData) *Cause {
	recentBackends := make([]string, len(recent))
	for i, d := range recent {
		recentBackends[i] = d.Backend
	}
	baselineBackends := make([]string, len(baseline))
	for i, d := range baseline {
		baselineBackends[i] = d.Backend
	}

	recentMost := getMostFrequent(recentBackends)
	baselineMost := getMostFrequent(baselineBackends)

	if recentMost != baselineMost && recentMost != "" && baselineMost != "" {
		changeCount := 0
		for _, b := range recentBackends {
			if b != baselineMost {
				changeCount++
			}
		}
		score := float64(changeCount) / float64(len(recentBackends))

		return &Cause{
			Dimension: "gpu_backend",
			OldValue:  baselineMost,
			NewValue:  recentMost,
			Score:     score,
		}
	}
	return nil
}

func (a *Analyzer) analyzeGPUDriverChange(recent, baseline []BenchmarkData) *Cause {
	recentDrivers := make([]string, len(recent))
	for i, d := range recent {
		recentDrivers[i] = d.GPUDriver
	}
	baselineDrivers := make([]string, len(baseline))
	for i, d := range baseline {
		baselineDrivers[i] = d.GPUDriver
	}

	recentMost := getMostFrequent(recentDrivers)
	baselineMost := getMostFrequent(baselineDrivers)

	if recentMost != baselineMost && recentMost != "" && baselineMost != "" {
		changeCount := 0
		for _, d := range recentDrivers {
			if d != baselineMost {
				changeCount++
			}
		}
		score := float64(changeCount) / float64(len(recentDrivers))

		return &Cause{
			Dimension: "gpu_driver",
			OldValue:  baselineMost,
			NewValue:  recentMost,
			Score:     score,
		}
	}
	return nil
}

func (c Cause) String() string {
	return fmt.Sprintf("[%s] %s -> %s (score=%.2f)", c.Dimension, c.OldValue, c.NewValue, c.Score)
}
