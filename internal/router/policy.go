package router

import "fmt"

func BuildPolicyKey(w Workload) string {
	return fmt.Sprintf("ws=%s|proj=%s|agent=%s|cat=%s|task=%s",
		w.WorkspaceID, w.ProjectID, w.AgentType, w.Category, w.TaskType)
}
