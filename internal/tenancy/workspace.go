package tenancy

import "errors"

type Workspace struct {
	ID   string
	Name string
	Org  string
	Role string
}

var ErrNoWorkspace = errors.New("no workspace")

// MVP: 단일 워크스페이스(로컬) 기본값
func DefaultWorkspace() Workspace {
	return Workspace{
		ID:   "local",
		Name: "Local Workspace",
		Org:  "local",
		Role: "owner",
	}
}
