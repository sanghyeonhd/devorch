package routes

import (
	"net/http"
	"strconv"
)

// TaskListHandler는 작업 목록 핸들러입니다
func TaskListHandler(deps *RouteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.BackgroundMgr == nil {
			writeError(w, http.StatusServiceUnavailable, "background manager not available")
			return
		}

		limit := 50
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		tasks, err := deps.BackgroundMgr.ListTasks(limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"tasks": tasks,
			"count": len(tasks),
		})
	}
}

// TaskGetHandler는 작업 조회 핸들러입니다
func TaskGetHandler(deps *RouteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.BackgroundMgr == nil {
			writeError(w, http.StatusServiceUnavailable, "background manager not available")
			return
		}

		taskID := r.PathValue("id")
		if taskID == "" {
			writeError(w, http.StatusBadRequest, "task id required")
			return
		}

		task, err := deps.BackgroundMgr.GetTask(taskID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, task)
	}
}

// TaskCancelHandler는 작업 취소 핸들러입니다
func TaskCancelHandler(deps *RouteDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.BackgroundMgr == nil {
			writeError(w, http.StatusServiceUnavailable, "background manager not available")
			return
		}

		taskID := r.PathValue("id")
		if taskID == "" {
			writeError(w, http.StatusBadRequest, "task id required")
			return
		}

		if err := deps.BackgroundMgr.CancelTask(taskID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "cancelled",
			"task_id": taskID,
		})
	}
}
