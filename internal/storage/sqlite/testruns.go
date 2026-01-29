package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/signal"
)

type TestRunStore struct {
	DB *sql.DB
}

func (s *TestRunStore) InsertTestRun(ctx context.Context, r signal.TestRun) (string, error) {
	_, err := s.DB.ExecContext(ctx, `
	  INSERT INTO test_runs (
	    id, workspace, project_id, session_id, task_id,
	    runner, command, started_at, ended_at, duration_ms,
	    success, exit_code, error, stdout_summary, stderr_summary, tags
	  ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		r.ID, r.Workspace, r.ProjectID, r.SessionID, r.TaskID,
		r.Runner, r.Command, r.StartedAt.UnixMilli(), r.EndedAt.UnixMilli(), r.DurationMs,
		boolToInt(r.Success), r.ExitCode, r.Error, r.StdoutSum, r.StderrSum, r.Tags,
	)
	return r.ID, err
}

func (s *TestRunStore) RecentRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.TestRun, error) {
	rows, err := s.DB.QueryContext(ctx, `
	  SELECT id, workspace, project_id, session_id, task_id,
	         runner, command, started_at, ended_at, duration_ms,
	         success, exit_code, error, stdout_summary, stderr_summary, tags
	  FROM test_runs
	  WHERE workspace=? AND (session_id=? OR ?="") AND (task_id=? OR ?="")
	  ORDER BY started_at DESC
	  LIMIT ?
	`, workspace, sessionID, sessionID, taskID, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []signal.TestRun
	for rows.Next() {
		var r signal.TestRun
		var startedMs, endedMs int64
		var successInt int
		if err := rows.Scan(
			&r.ID, &r.Workspace, &r.ProjectID, &r.SessionID, &r.TaskID,
			&r.Runner, &r.Command, &startedMs, &endedMs, &r.DurationMs,
			&successInt, &r.ExitCode, &r.Error, &r.StdoutSum, &r.StderrSum, &r.Tags,
		); err != nil {
			return nil, err
		}
		r.Success = successInt == 1
		r.StartedAt = time.UnixMilli(startedMs)
		r.EndedAt = time.UnixMilli(endedMs)
		out = append(out, r)
	}
	return out, nil
}
