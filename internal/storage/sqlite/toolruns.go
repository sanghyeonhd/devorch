package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/signal"
)

type ToolRunStore struct {
	DB *sql.DB
}

func (s *ToolRunStore) InsertToolRun(ctx context.Context, r signal.ToolRun) (string, error) {
	_, err := s.DB.ExecContext(ctx, `
	  INSERT INTO tool_runs (
	    id, workspace, session_id, task_id,
	    tool_name, started_at, ended_at, duration_ms,
	    success, error, input_summary, output_summary, output_bytes, tags,
	    exit_code, meta_json
	  ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		r.ID, r.Workspace, r.SessionID, r.TaskID,
		r.ToolName, r.StartedAt.UnixMilli(), r.EndedAt.UnixMilli(), r.DurationMs,
		boolToInt(r.Success), r.Error, r.InputSummary, r.OutputSummary, r.OutputBytes, r.Tags,
		r.ExitCode, r.MetaJSON,
	)
	return r.ID, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type ToolAgg struct {
	ToolName string
	Count    int
	FailRate float64
	AvgMs    float64
}

func (s *ToolRunStore) RecentToolAgg(ctx context.Context, workspace string, limit int) ([]ToolAgg, error) {
	rows, err := s.DB.QueryContext(ctx, `
	  SELECT tool_name,
	         COUNT(*) as cnt,
	         AVG(CASE WHEN success=1 THEN 0 ELSE 1 END) as fail_rate,
	         AVG(duration_ms) as avg_ms
	  FROM tool_runs
	  WHERE workspace=?
	  GROUP BY tool_name
	  ORDER BY cnt DESC
	  LIMIT ?
	`, workspace, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ToolAgg{}
	for rows.Next() {
		var a ToolAgg
		if err := rows.Scan(&a.ToolName, &a.Count, &a.FailRate, &a.AvgMs); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *ToolRunStore) RecentRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.ToolRun, error) {
	rows, err := s.DB.QueryContext(ctx, `
	  SELECT id, workspace, session_id, task_id, tool_name,
	         started_at, ended_at, duration_ms, success, error,
	         input_summary, output_summary, output_bytes, tags,
	         COALESCE(exit_code, 0), COALESCE(meta_json, '')
	  FROM tool_runs
	  WHERE workspace=? AND (session_id=? OR ?="") AND (task_id=? OR ?="")
	  ORDER BY started_at DESC
	  LIMIT ?
	`, workspace, sessionID, sessionID, taskID, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []signal.ToolRun
	for rows.Next() {
		var r signal.ToolRun
		var startedMs, endedMs int64
		var successInt int
		if err := rows.Scan(
			&r.ID, &r.Workspace, &r.SessionID, &r.TaskID, &r.ToolName,
			&startedMs, &endedMs, &r.DurationMs, &successInt, &r.Error,
			&r.InputSummary, &r.OutputSummary, &r.OutputBytes, &r.Tags,
			&r.ExitCode, &r.MetaJSON,
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
