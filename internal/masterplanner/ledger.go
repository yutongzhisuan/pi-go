package masterplanner

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
    task_id              TEXT PRIMARY KEY,
    run_id               TEXT NOT NULL,
    batch_id             TEXT NOT NULL DEFAULT '',
    goal                 TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'submitted',
    cursor_event_id      TEXT NOT NULL DEFAULT '',
    gateway_instance_id  TEXT NOT NULL DEFAULT '',
    submitted_at         REAL NOT NULL,
    updated_at           REAL NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tasks_run_id ON tasks(run_id);
CREATE INDEX IF NOT EXISTS idx_tasks_batch_id ON tasks(batch_id);
`

// Ledger is a thread-safe SQLite task ledger.
type Ledger struct {
	dbPath string
	db     *sql.DB
	mu     sync.Mutex
}

// NewLedger creates or opens a task ledger at the given path.
// If dbPath is empty, uses $PI_HOME/masterplanner.db (default ~/.pi-go/).
// Pass ":memory:" for in-memory testing databases.
func NewLedger(dbPath string) (*Ledger, error) {
	if dbPath == "" {
		home := os.Getenv("PI_HOME")
		if home == "" {
			home = filepath.Join(os.Getenv("HOME"), ".pi-go")
		}
		dbPath = filepath.Join(home, "masterplanner.db")
	}

	if dbPath != ":memory:" {
		parent := filepath.Dir(dbPath)
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir ledger parent: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open ledger: %w", err)
	}

	l := &Ledger{dbPath: dbPath, db: db}
	if err := l.init(); err != nil {
		db.Close()
		return nil, err
	}
	return l, nil
}

func (l *Ledger) init() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.db.Exec(schema); err != nil {
		return fmt.Errorf("init ledger schema: %w", err)
	}
	return nil
}

// Close closes the ledger database.
func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.db.Close()
}

// Record inserts or idempotently replaces a task row at dispatch time.
func (l *Ledger) Record(runID, taskID, goal string, opts ...RecordOption) error {
	cfg := recordConfig{
		batchID:           "",
		status:            TaskStatusPending,
		gatewayInstanceID: "",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	now := time.Now().Unix()
	l.mu.Lock()
	defer l.mu.Unlock()

	const q = `
		INSERT INTO tasks (task_id, run_id, batch_id, goal, status, gateway_instance_id, submitted_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			status = excluded.status,
			gateway_instance_id = excluded.gateway_instance_id,
			updated_at = excluded.updated_at
	`
	_, err := l.db.Exec(q, taskID, runID, cfg.batchID, goal, string(cfg.status), cfg.gatewayInstanceID, now, now)
	if err != nil {
		return fmt.Errorf("record task: %w", err)
	}
	return nil
}

type recordConfig struct {
	batchID           string
	status            TaskStatus
	gatewayInstanceID string
}

// RecordOption customizes Record behavior.
type RecordOption func(*recordConfig)

// WithBatchID sets the batch ID for the task.
func WithBatchID(batchID string) RecordOption {
	return func(c *recordConfig) { c.batchID = batchID }
}

// WithStatus sets the initial status for the task.
func WithStatus(status TaskStatus) RecordOption {
	return func(c *recordConfig) { c.status = status }
}

// WithGatewayInstanceID sets the gateway instance ID for the task.
func WithGatewayInstanceID(instanceID string) RecordOption {
	return func(c *recordConfig) { c.gatewayInstanceID = instanceID }
}

// UpdateStatus updates the status of a task.
func (l *Ledger) UpdateStatus(taskID string, status TaskStatus) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	const q = `UPDATE tasks SET status = ?, updated_at = ? WHERE task_id = ?`
	_, err := l.db.Exec(q, string(status), time.Now().Unix(), taskID)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}

// UpdateCursor persists the watch resume cursor, segmented by gateway instance.
func (l *Ledger) UpdateCursor(taskID, cursorEventID, gatewayInstanceID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().Unix()
	var q string
	var args []any

	if gatewayInstanceID != "" {
		q = `UPDATE tasks SET cursor_event_id = ?, gateway_instance_id = ?, updated_at = ? WHERE task_id = ?`
		args = []any{cursorEventID, gatewayInstanceID, now, taskID}
	} else {
		q = `UPDATE tasks SET cursor_event_id = ?, updated_at = ? WHERE task_id = ?`
		args = []any{cursorEventID, now, taskID}
	}

	_, err := l.db.Exec(q, args...)
	if err != nil {
		return fmt.Errorf("update cursor: %w", err)
	}
	return nil
}

// NextSeq returns the next monotonic sequence number for the given run ID.
func (l *Ledger) NextSeq(runID string) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	const q = `SELECT COUNT(*) FROM tasks WHERE run_id = ?`
	var n int
	if err := l.db.QueryRow(q, runID).Scan(&n); err != nil {
		return 0, fmt.Errorf("next seq: %w", err)
	}
	return n + 1, nil
}

// Get retrieves a single task record by ID.
func (l *Ledger) Get(taskID string) (*TaskRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	const q = `SELECT task_id, run_id, batch_id, goal, status, cursor_event_id, gateway_instance_id, submitted_at, updated_at FROM tasks WHERE task_id = ?`
	row := l.db.QueryRow(q, taskID)

	var rec TaskRecord
	var submittedSec, updatedSec float64
	err := row.Scan(&rec.TaskID, &rec.RunID, &rec.BatchID, &rec.Goal, &rec.Status, &rec.CursorEventID, &rec.GatewayInstanceID, &submittedSec, &updatedSec)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	rec.SubmittedAt = time.Unix(int64(submittedSec), 0)
	rec.UpdatedAt = time.Unix(int64(updatedSec), 0)
	return &rec, nil
}

// OpenTasks lists non-terminal tasks, optionally filtered by run ID.
func (l *Ledger) OpenTasks(runID string) ([]TaskRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	terminalList := []string{
		string(TaskStatusCompleted),
		string(TaskStatusFailed),
		string(TaskStatusLost),
		string(TaskStatusCancelled),
	}
	placeholders := "?, ?, ?, ?"
	args := make([]any, 0, len(terminalList)+1)
	for _, s := range terminalList {
		args = append(args, s)
	}

	q := fmt.Sprintf(`SELECT task_id, run_id, batch_id, goal, status, cursor_event_id, gateway_instance_id, submitted_at, updated_at FROM tasks WHERE status NOT IN (%s)`, placeholders)
	if runID != "" {
		q += ` AND run_id = ?`
		args = append(args, runID)
	}
	q += ` ORDER BY submitted_at`

	rows, err := l.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("open tasks: %w", err)
	}
	defer rows.Close()

	var recs []TaskRecord
	for rows.Next() {
		var rec TaskRecord
		var submittedSec, updatedSec float64
		if err := rows.Scan(&rec.TaskID, &rec.RunID, &rec.BatchID, &rec.Goal, &rec.Status, &rec.CursorEventID, &rec.GatewayInstanceID, &submittedSec, &updatedSec); err != nil {
			return nil, fmt.Errorf("scan open task: %w", err)
		}
		rec.SubmittedAt = time.Unix(int64(submittedSec), 0)
		rec.UpdatedAt = time.Unix(int64(updatedSec), 0)
		recs = append(recs, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("open tasks rows: %w", err)
	}
	return recs, nil
}

// TasksInBatch lists all tasks in a batch, ordered by submission time.
func (l *Ledger) TasksInBatch(batchID string) ([]TaskRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	const q = `SELECT task_id, run_id, batch_id, goal, status, cursor_event_id, gateway_instance_id, submitted_at, updated_at FROM tasks WHERE batch_id = ? ORDER BY submitted_at`
	rows, err := l.db.Query(q, batchID)
	if err != nil {
		return nil, fmt.Errorf("tasks in batch: %w", err)
	}
	defer rows.Close()

	var recs []TaskRecord
	for rows.Next() {
		var rec TaskRecord
		var submittedSec, updatedSec float64
		if err := rows.Scan(&rec.TaskID, &rec.RunID, &rec.BatchID, &rec.Goal, &rec.Status, &rec.CursorEventID, &rec.GatewayInstanceID, &submittedSec, &updatedSec); err != nil {
			return nil, fmt.Errorf("scan batch task: %w", err)
		}
		rec.SubmittedAt = time.Unix(int64(submittedSec), 0)
		rec.UpdatedAt = time.Unix(int64(updatedSec), 0)
		recs = append(recs, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tasks in batch rows: %w", err)
	}
	return recs, nil
}
