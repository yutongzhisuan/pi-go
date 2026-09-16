package masterplanner

import (
	"testing"
	"time"
)

func TestLedger_RecordAndGet(t *testing.T) {
	l, err := NewLedger(":memory:")
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	defer l.Close()

	runID := "run-1"
	taskID := "task-1"
	goal := "test goal"

	if err := l.Record(runID, taskID, goal, WithBatchID("batch-1"), WithStatus(TaskStatusPending)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	rec, err := l.Get(taskID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("Get returned nil")
	}
	if rec.TaskID != taskID {
		t.Errorf("TaskID = %q, want %q", rec.TaskID, taskID)
	}
	if rec.RunID != runID {
		t.Errorf("RunID = %q, want %q", rec.RunID, runID)
	}
	if rec.BatchID != "batch-1" {
		t.Errorf("BatchID = %q, want %q", rec.BatchID, "batch-1")
	}
	if rec.Goal != goal {
		t.Errorf("Goal = %q, want %q", rec.Goal, goal)
	}
	if rec.Status != TaskStatusPending {
		t.Errorf("Status = %q, want %q", rec.Status, TaskStatusPending)
	}
}

func TestLedger_UpdateStatus(t *testing.T) {
	l, err := NewLedger(":memory:")
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	defer l.Close()

	taskID := "task-1"
	if err := l.Record("run-1", taskID, "goal", WithStatus(TaskStatusPending)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if err := l.UpdateStatus(taskID, TaskStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	rec, err := l.Get(taskID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.Status != TaskStatusCompleted {
		t.Errorf("Status = %q, want %q", rec.Status, TaskStatusCompleted)
	}
}

func TestLedger_UpdateCursor(t *testing.T) {
	l, err := NewLedger(":memory:")
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	defer l.Close()

	taskID := "task-1"
	if err := l.Record("run-1", taskID, "goal"); err != nil {
		t.Fatalf("Record: %v", err)
	}

	cursor := "cursor-123"
	instanceID := "instance-456"
	if err := l.UpdateCursor(taskID, cursor, instanceID); err != nil {
		t.Fatalf("UpdateCursor: %v", err)
	}

	rec, err := l.Get(taskID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.CursorEventID != cursor {
		t.Errorf("CursorEventID = %q, want %q", rec.CursorEventID, cursor)
	}
	if rec.GatewayInstanceID != instanceID {
		t.Errorf("GatewayInstanceID = %q, want %q", rec.GatewayInstanceID, instanceID)
	}
}

func TestLedger_NextSeq(t *testing.T) {
	l, err := NewLedger(":memory:")
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	defer l.Close()

	runID := "run-1"

	seq1, err := l.NextSeq(runID)
	if err != nil {
		t.Fatalf("NextSeq(1): %v", err)
	}
	if seq1 != 1 {
		t.Errorf("seq1 = %d, want 1", seq1)
	}

	if err := l.Record(runID, "task-1", "goal"); err != nil {
		t.Fatalf("Record: %v", err)
	}

	seq2, err := l.NextSeq(runID)
	if err != nil {
		t.Fatalf("NextSeq(2): %v", err)
	}
	if seq2 != 2 {
		t.Errorf("seq2 = %d, want 2", seq2)
	}

	if err := l.Record(runID, "task-2", "goal"); err != nil {
		t.Fatalf("Record: %v", err)
	}

	seq3, err := l.NextSeq(runID)
	if err != nil {
		t.Fatalf("NextSeq(3): %v", err)
	}
	if seq3 != 3 {
		t.Errorf("seq3 = %d, want 3", seq3)
	}
}

func TestLedger_OpenTasks(t *testing.T) {
	l, err := NewLedger(":memory:")
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	defer l.Close()

	runID := "run-1"
	if err := l.Record(runID, "task-1", "goal1", WithStatus(TaskStatusPending)); err != nil {
		t.Fatalf("Record task-1: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := l.Record(runID, "task-2", "goal2", WithStatus(TaskStatusRunning)); err != nil {
		t.Fatalf("Record task-2: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := l.Record(runID, "task-3", "goal3", WithStatus(TaskStatusCompleted)); err != nil {
		t.Fatalf("Record task-3: %v", err)
	}

	open, err := l.OpenTasks(runID)
	if err != nil {
		t.Fatalf("OpenTasks: %v", err)
	}
	if len(open) != 2 {
		t.Fatalf("len(open) = %d, want 2", len(open))
	}
	if open[0].TaskID != "task-1" {
		t.Errorf("open[0].TaskID = %q, want task-1", open[0].TaskID)
	}
	if open[1].TaskID != "task-2" {
		t.Errorf("open[1].TaskID = %q, want task-2", open[1].TaskID)
	}
}

func TestLedger_TasksInBatch(t *testing.T) {
	l, err := NewLedger(":memory:")
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	defer l.Close()

	batchID := "batch-1"
	if err := l.Record("run-1", "task-1", "goal1", WithBatchID(batchID)); err != nil {
		t.Fatalf("Record task-1: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := l.Record("run-1", "task-2", "goal2", WithBatchID(batchID)); err != nil {
		t.Fatalf("Record task-2: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := l.Record("run-1", "task-3", "goal3", WithBatchID("batch-2")); err != nil {
		t.Fatalf("Record task-3: %v", err)
	}

	batch, err := l.TasksInBatch(batchID)
	if err != nil {
		t.Fatalf("TasksInBatch: %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("len(batch) = %d, want 2", len(batch))
	}
	if batch[0].TaskID != "task-1" {
		t.Errorf("batch[0].TaskID = %q, want task-1", batch[0].TaskID)
	}
	if batch[1].TaskID != "task-2" {
		t.Errorf("batch[1].TaskID = %q, want task-2", batch[1].TaskID)
	}
}

func TestLedger_Idempotent(t *testing.T) {
	l, err := NewLedger(":memory:")
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	defer l.Close()

	taskID := "task-1"
	if err := l.Record("run-1", taskID, "goal1", WithStatus(TaskStatusPending)); err != nil {
		t.Fatalf("Record(1): %v", err)
	}

	rec1, err := l.Get(taskID)
	if err != nil {
		t.Fatalf("Get(1): %v", err)
	}

	if err := l.Record("run-1", taskID, "goal2", WithStatus(TaskStatusRunning)); err != nil {
		t.Fatalf("Record(2): %v", err)
	}

	rec2, err := l.Get(taskID)
	if err != nil {
		t.Fatalf("Get(2): %v", err)
	}

	if rec2.Status != TaskStatusRunning {
		t.Errorf("Status = %q after re-record, want running", rec2.Status)
	}
	if rec1.Goal != "goal1" {
		t.Errorf("First goal = %q, want goal1", rec1.Goal)
	}
}

func TestLedger_ConcurrentAccess(t *testing.T) {
	l, err := NewLedger(":memory:")
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	defer l.Close()

	const n = 50
	done := make(chan bool, n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer func() { done <- true }()
			taskID := "task-" + string(rune('A'+i%26))
			if err := l.Record("run-1", taskID, "goal", WithStatus(TaskStatusPending)); err != nil {
				t.Errorf("Record %s: %v", taskID, err)
			}
			if _, err := l.Get(taskID); err != nil {
				t.Errorf("Get %s: %v", taskID, err)
			}
		}(i)
	}

	for i := 0; i < n; i++ {
		<-done
	}
}
