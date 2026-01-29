package learning

import (
	"context"
	"errors"
	"time"

	"devorch/internal/id"
)

type FeedbackStore interface {
	InsertFeedback(ctx context.Context, f Feedback) error
}

type Feedback struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`

	// 무엇에 대한 피드백인지(세션/메시지/작업)
	SessionID string `json:"sessionId"`
	MessageID string `json:"messageId"`

	Provider string `json:"provider"`
	Model    string `json:"model"`
	Scenario string `json:"scenario"`

	// thumbsUp: true면 +, false면 -
	ThumbsUp bool   `json:"thumbsUp"`
	Note     string `json:"note,omitempty"`

	// 0..1로 변환된 quality (라우터 학습용)
	QualityScore float64 `json:"qualityScore"`
}

func ThumbsToQuality(thumbsUp bool) float64 {
	if thumbsUp {
		return 1.0
	}
	return 0.0
}

type FeedbackService struct {
	Store FeedbackStore
	Bench *Recorder // 피드백이 들어오면 "qualityScore"를 benchmarks에도 반영할 수 있음(선택)
}

func NewFeedbackService(store FeedbackStore) *FeedbackService {
	return &FeedbackService{Store: store}
}

func (s *FeedbackService) Submit(ctx context.Context, in Feedback) (Feedback, error) {
	if s.Store == nil {
		return Feedback{}, errors.New("feedback store not configured")
	}
	if in.ID == "" {
		in.ID = id.NewULID()
	}
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now()
	}
	if in.QualityScore < 0 {
		in.QualityScore = 0
	}
	if in.QualityScore > 1 {
		in.QualityScore = 1
	}
	if err := s.Store.InsertFeedback(ctx, in); err != nil {
		return Feedback{}, err
	}
	return in, nil
}

// Helper: thumbs 기반 입력이면 qualityScore 자동 채움
func NewThumbFeedback(sessionID, messageID, provider, model, scenario string, thumbsUp bool, note string) Feedback {
	return Feedback{
		SessionID:    sessionID,
		MessageID:    messageID,
		Provider:     provider,
		Model:        model,
		Scenario:     scenario,
		ThumbsUp:     thumbsUp,
		Note:         note,
		QualityScore: ThumbsToQuality(thumbsUp),
	}
}
