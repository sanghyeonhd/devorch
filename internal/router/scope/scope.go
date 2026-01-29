package scope

type Type string

const (
	Global    Type = "global"
	Org       Type = "org"
	Workspace Type = "workspace"
	User      Type = "user"
	Project   Type = "project"
)

type Key struct {
	Type Type
	ID   string // global = "*"
}

func GlobalKey() Key {
	return Key{Type: Global, ID: "*"}
}
