package http

import (
	"encoding/json"
	"net/http"

	"devorch/internal/learning"
)

type FeedbackRoutes struct {
	Svc *learning.FeedbackService
}

func NewFeedbackRoutes(svc *learning.FeedbackService) *FeedbackRoutes {
	return &FeedbackRoutes{Svc: svc}
}

func (r *FeedbackRoutes) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/feedback", r.postFeedback)
}

func (r *FeedbackRoutes) postFeedback(w http.ResponseWriter, req *http.Request) {
	var payload struct {
		SessionID    string  `json:"session_id"`
		MessageID    string  `json:"message_id"`
		Provider     string  `json:"provider"`
		Model        string  `json:"model"`
		Scenario     string  `json:"scenario"`
		ThumbsUp     bool    `json:"thumbs_up"`
		Note         string  `json:"note,omitempty"`
		QualityScore float64 `json:"quality_score,omitempty"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fb, err := r.Svc.Submit(req.Context(), learning.Feedback{
		SessionID:    payload.SessionID,
		MessageID:    payload.MessageID,
		Provider:     payload.Provider,
		Model:        payload.Model,
		Scenario:     payload.Scenario,
		ThumbsUp:     payload.ThumbsUp,
		Note:         payload.Note,
		QualityScore: payload.QualityScore,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": fb.ID})
}
