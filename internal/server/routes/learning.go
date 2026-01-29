package routes

import (
	"net/http"
	"strconv"
)

// LearningArmsHandler는 학습 Arms 핸들러입니다
func LearningArmsHandler(deps *RouteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.OkaonStore == nil {
			writeError(w, http.StatusServiceUnavailable, "okaon store not available")
			return
		}

		limit := 100
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		stats, err := deps.OkaonStore.QueryArmStats(limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"arms":  stats,
			"count": len(stats),
		})
	}
}

// LearningRunsHandler는 학습 Runs 핸들러입니다
func LearningRunsHandler(deps *RouteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.OkaonStore == nil {
			writeError(w, http.StatusServiceUnavailable, "okaon store not available")
			return
		}

		limit := 100
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		runs, err := deps.OkaonStore.QueryRuns(limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"runs":  runs,
			"count": len(runs),
		})
	}
}

// LearningStatsHandler는 학습 통계 핸들러입니다
func LearningStatsHandler(deps *RouteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.OkaonStore == nil {
			writeError(w, http.StatusServiceUnavailable, "okaon store not available")
			return
		}

		arms, errArms := deps.OkaonStore.QueryArmStats(1000)
		runs, errRuns := deps.OkaonStore.QueryRuns(1000)

		if errArms != nil && errRuns != nil {
			writeError(w, http.StatusInternalServerError, "failed to query stats")
			return
		}

		// 통계 계산
		totalRuns := len(runs)
		totalArms := len(arms)

		var avgMean float64
		if totalArms > 0 {
			var sum float64
			for _, arm := range arms {
				sum += arm.Mean
			}
			avgMean = sum / float64(totalArms)
		}

		// 프로바이더별 분포
		providerDist := make(map[string]int)
		for _, run := range runs {
			providerDist[run.Provider]++
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"total_runs":            totalRuns,
			"total_arms":            totalArms,
			"avg_mean":              avgMean,
			"provider_distribution": providerDist,
		})
	}
}
