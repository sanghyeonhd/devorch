package builtins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"devorch/internal/id"
	"devorch/internal/okaon"
)

// 작업 완료 시 OkAON에 work/run 기본 기록
// 실제 daemon의 tool/task 실행 완료 이벤트에서 호출되는 형태를 가정
type TaskCompleteInput struct {
	ProjectID   string
	WorkspaceID string
	UserID      string

	TaskType      string
	Category      string
	Agent         string
	Prompt        string // 원문 저장 금지(해시만)
	Context       string // 원문 저장 금지(해시만)
	OS            string
	Arch          string
	HWFingerprint string
	TagsJSON      string

	Provider    string
	Model       string
	RoutePolicy string
	WasFallback bool
	RetryCount  int

	LatencyMs    int64
	InputTokens  int64
	OutputTokens int64
	CostMicroUSD int64

	ErrorCode string
	ErrorMsg  string
}

type OkStore interface {
	InsertWork(ctx context.Context, w okaon.Work) error
	InsertRun(ctx context.Context, r okaon.Run) error
}

func OnTaskComplete(ctx context.Context, store OkStore, in TaskCompleteInput) (workID, runID string, err error) {
	now := time.Now().UTC()

	workID = id.NewULID()
	runID = id.NewULID()

	w := okaon.Work{
		ID:          workID,
		ProjectID:   in.ProjectID,
		WorkspaceID: in.WorkspaceID,
		UserID:      in.UserID,
		TaskType:    in.TaskType,
		Category:    in.Category,
		Agent:       in.Agent,
		PromptHash:  hashText(in.Prompt),
		ContextHash: hashText(in.Context),
		OS:          in.OS,
		Arch:        in.Arch,
		Fingerprint: in.HWFingerprint,
		TagsJSON:    in.TagsJSON,
		CreatedAt:   now,
	}

	r := okaon.Run{
		ID:           runID,
		WorkID:       workID,
		Provider:     in.Provider,
		Model:        in.Model,
		RoutePolicy:  in.RoutePolicy,
		WasFallback:  in.WasFallback,
		RetryCount:   in.RetryCount,
		LatencyMs:    in.LatencyMs,
		InputTokens:  in.InputTokens,
		OutputTokens: in.OutputTokens,
		CostMicroUSD: in.CostMicroUSD,
		ErrorCode:    in.ErrorCode,
		ErrorMsg:     in.ErrorMsg,
		CreatedAt:    now,
	}

	if err := store.InsertWork(ctx, w); err != nil {
		return "", "", err
	}
	if err := store.InsertRun(ctx, r); err != nil {
		return "", "", err
	}
	return workID, runID, nil
}

func hashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
