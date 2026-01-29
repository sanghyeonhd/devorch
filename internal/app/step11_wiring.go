package app

import (
	"context"
	"database/sql"

	"devorch/internal/learning"
	learninghttp "devorch/internal/learning/http"
	"devorch/internal/provider/observer"
	"devorch/internal/router"
	routerhttp "devorch/internal/router/http"
	"devorch/internal/storage/sqlite"
)

// Step11Deps: Phase 11에서 추가된 의존성 조립
type Step11Deps struct {
	// Stores
	FeedbackStore *sqlite.FeedbackStore

	// Services
	FeedbackService *learning.FeedbackService
	Recorder        *learning.Recorder

	// Observer chain
	ObserverChain observer.Chain

	// Policy router
	PolicyStore  *router.PolicyStore
	PolicyRouter *router.PolicyRouter

	// HTTP routes
	FeedbackRoutes *learninghttp.FeedbackRoutes
	RouterRoutes   *routerhttp.RouterRoutes
}

// feedbackStoreAdapter: sqlite.FeedbackStore → learning.FeedbackStore 어댑터
type feedbackStoreAdapter struct {
	store *sqlite.FeedbackStore
}

func (a *feedbackStoreAdapter) InsertFeedback(ctx context.Context, f learning.Feedback) error {
	row := sqlite.FeedbackRow{
		ID:           f.ID,
		CreatedAt:    f.CreatedAt,
		SessionID:    f.SessionID,
		MessageID:    f.MessageID,
		Provider:     f.Provider,
		Model:        f.Model,
		Scenario:     f.Scenario,
		ThumbsUp:     f.ThumbsUp,
		Note:         f.Note,
		QualityScore: f.QualityScore,
	}
	return a.store.InsertFeedback(ctx, row)
}

// benchStoreAdapter: sqlite.BenchmarksStore → learning.BenchmarksStore 어댑터
type benchStoreAdapter struct {
	store *sqlite.BenchmarksStore
}

func (a *benchStoreAdapter) Insert(ctx context.Context, row learning.BenchmarkRow) error {
	sqlRow := sqlite.BenchmarkRow{
		ID:                 row.ID,
		CreatedAt:          row.CreatedAt,
		OS:                 row.OS,
		Arch:               row.Arch,
		MachineFingerprint: row.MachineFingerprint,
		Provider:           row.Provider,
		Model:              row.Model,
		Scenario:           row.Scenario,
		InputTokens:        row.InputTokens,
		OutputTokens:       row.OutputTokens,
		LatencyMs:          row.LatencyMs,
		Success:            row.Success,
		Error:              row.Error,
		QualityScore:       row.QualityScore,
		CostUSD:            row.CostUSD,
		Metadata:           row.Metadata,
	}
	return a.store.Insert(ctx, sqlRow)
}

func WireStep11(db *sql.DB, benchStore *sqlite.BenchmarksStore) *Step11Deps {
	deps := &Step11Deps{}

	// Storage layer
	deps.FeedbackStore = sqlite.NewFeedbackStore(db)

	// Adapters
	feedbackAdapter := &feedbackStoreAdapter{store: deps.FeedbackStore}
	benchAdapter := &benchStoreAdapter{store: benchStore}

	// Learning services
	deps.FeedbackService = learning.NewFeedbackService(feedbackAdapter)
	deps.Recorder = learning.NewRecorder(benchAdapter)

	// Observer chain
	benchObs := observer.NewBenchRecorderObserver(deps.Recorder)
	deps.ObserverChain = observer.Chain{Items: []observer.Observer{benchObs}}

	// Router
	deps.PolicyStore = router.NewPolicyStore(db)
	deps.PolicyRouter = router.NewPolicyRouter(deps.PolicyStore)

	// HTTP
	deps.FeedbackRoutes = learninghttp.NewFeedbackRoutes(deps.FeedbackService)
	deps.RouterRoutes = routerhttp.NewRouterRoutes(deps.PolicyRouter, deps.PolicyStore)

	return deps
}
