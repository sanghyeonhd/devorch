package scope

import "context"

type CtxKey string

const (
	CtxOrgID       CtxKey = "org_id"
	CtxWorkspaceID CtxKey = "workspace_id"
	CtxUserID      CtxKey = "user_id"
	CtxProjectID   CtxKey = "project_id"
)

// 요청 컨텍스트를 기반으로, 라우팅에 사용할 scope 후보를 만든다.
// 우선순위: user > workspace > org > global
func Candidates(ctx context.Context) []Key {
	var out []Key

	if v := ctx.Value(CtxUserID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, Key{Type: User, ID: s})
		}
	}
	if v := ctx.Value(CtxWorkspaceID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, Key{Type: Workspace, ID: s})
		}
	}
	if v := ctx.Value(CtxOrgID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, Key{Type: Org, ID: s})
		}
	}

	out = append(out, GlobalKey())
	return out
}
