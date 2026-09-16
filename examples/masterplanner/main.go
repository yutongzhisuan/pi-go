package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dimetron/pi-go/internal/masterplanner"
)

func main() {
	baseURL := os.Getenv("INFA_GATEWAY_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:18082"
	}

	apiKey := os.Getenv("INFA_GATEWAY_API_KEY")

	client := masterplanner.NewClient(
		masterplanner.WithBaseURL(baseURL),
		masterplanner.WithAPIKey(apiKey),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	if err := run(ctx, client); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run(ctx context.Context, client *masterplanner.Client) error {
	fmt.Println("=== Gateway API Master Planner Example ===\n")

	if err := listModels(ctx, client); err != nil {
		return fmt.Errorf("list models: %w", err)
	}

	if err := listWorkers(ctx, client); err != nil {
		return fmt.Errorf("list workers: %w", err)
	}

	taskID, err := createTask(ctx, client)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	if err := watchTaskEvents(ctx, client, taskID); err != nil {
		return fmt.Errorf("watch events: %w", err)
	}

	if err := simulateTaskExecution(ctx, client, taskID); err != nil {
		return fmt.Errorf("simulate execution: %w", err)
	}

	if err := listTasks(ctx, client); err != nil {
		return fmt.Errorf("list tasks: %w", err)
	}

	return nil
}

func listModels(ctx context.Context, client *masterplanner.Client) error {
	fmt.Println("1. Listing available models...")

	resp, err := client.ListModels(ctx, &masterplanner.ListModelsRequest{
		PageSize: 10,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Found %d models:\n", len(resp.Models))
	for i, model := range resp.Models {
		fmt.Printf("  %d. %s (%s) - %d tokens\n",
			i+1, model.ModelID, model.Provider, model.ContextWindow)
	}
	fmt.Println()

	return nil
}

func listWorkers(ctx context.Context, client *masterplanner.Client) error {
	fmt.Println("2. Listing active workers...")

	resp, err := client.ListWorkers(ctx, &masterplanner.ListWorkersRequest{
		Status:   "active",
		PageSize: 10,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Found %d active workers:\n", len(resp.Workers))
	for i, worker := range resp.Workers {
		fmt.Printf("  %d. %s - Last seen: %s\n",
			i+1, worker.WorkerID, worker.LastSeenAt.Format(time.RFC3339))
	}
	fmt.Println()

	return nil
}

func createTask(ctx context.Context, client *masterplanner.Client) (string, error) {
	fmt.Println("3. Creating a new task...")

	resp, err := client.CreateTask(ctx, &masterplanner.CreateTaskRequest{
		WorkerID: "worker-1",
		ModelID:  "gpt-4",
		Prompt:   "Write a function to calculate fibonacci numbers",
		Priority: masterplanner.TaskPriorityNormal,
		Metadata: map[string]string{
			"user_id":   "user-123",
			"session":   "example-session",
			"timestamp": time.Now().Format(time.RFC3339),
		},
		Timeout:    3600,
		MaxRetries: 3,
	})
	if err != nil {
		return "", err
	}

	fmt.Printf("Task created: %s (status: %s)\n", resp.TaskID, resp.Status)
	fmt.Println()

	return resp.TaskID, nil
}

func watchTaskEvents(ctx context.Context, client *masterplanner.Client, taskID string) error {
	fmt.Println("4. Watching task events (SSE stream)...")

	watchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	stream, err := client.WatchTasks(watchCtx, &masterplanner.WatchTasksRequest{})
	if err != nil {
		return err
	}
	defer stream.Close()

	eventCount := 0
	for {
		select {
		case event := <-stream.Events:
			if event == nil {
				fmt.Printf("Received %d events\n\n", eventCount)
				return nil
			}
			eventCount++
			fmt.Printf("  Event: %s - %s (task: %s)\n",
				event.Kind, event.Message, event.TaskID)

		case err := <-stream.Errors:
			if err != nil {
				return err
			}
			fmt.Printf("Received %d events\n\n", eventCount)
			return nil

		case <-watchCtx.Done():
			fmt.Printf("Received %d events (timeout)\n\n", eventCount)
			return nil
		}
	}
}

func simulateTaskExecution(ctx context.Context, client *masterplanner.Client, taskID string) error {
	fmt.Println("5. Simulating task execution...")

	time.Sleep(1 * time.Second)

	result := `def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)`

	resp, err := client.SubmitTaskResult(ctx, taskID, &masterplanner.SubmitTaskResultRequest{
		TaskID: taskID,
		Result: result,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Task %s completed at %s\n", resp.TaskID, resp.CompletedAt.Format(time.RFC3339))
	fmt.Println()

	return nil
}

func listTasks(ctx context.Context, client *masterplanner.Client) error {
	fmt.Println("6. Listing all tasks...")

	resp, err := client.ListTasks(ctx, &masterplanner.ListTasksRequest{
		PageSize: 10,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Found %d tasks:\n", len(resp.Tasks))
	for i, task := range resp.Tasks {
		fmt.Printf("  %d. %s - %s (model: %s)\n",
			i+1, task.TaskID, task.Status, task.ModelID)
		if task.Result != "" {
			resultPreview := task.Result
			if len(resultPreview) > 50 {
				resultPreview = resultPreview[:50] + "..."
			}
			fmt.Printf("     Result: %s\n", resultPreview)
		}
	}
	fmt.Println()

	return nil
}

func batchCreateExample(ctx context.Context, client *masterplanner.Client) error {
	fmt.Println("7. Batch creating tasks...")

	resp, err := client.BatchCreateTasks(ctx, &masterplanner.BatchCreateTasksRequest{
		Tasks: []*masterplanner.CreateTaskRequest{
			{
				ModelID:  "gpt-4",
				Prompt:   "Explain quantum computing",
				Priority: masterplanner.TaskPriorityHigh,
			},
			{
				ModelID:  "claude-3",
				Prompt:   "Write a sorting algorithm",
				Priority: masterplanner.TaskPriorityNormal,
			},
			{
				ModelID:  "gemini-pro",
				Prompt:   "Summarize this article",
				Priority: masterplanner.TaskPriorityLow,
			},
		},
	})
	if err != nil {
		return err
	}

	fmt.Printf("Created %d tasks:\n", len(resp.Tasks))
	for i, task := range resp.Tasks {
		fmt.Printf("  %d. %s - %s\n", i+1, task.TaskID, task.Status)
	}
	fmt.Println()

	return nil
}

func cancelTaskExample(ctx context.Context, client *masterplanner.Client, taskID string) error {
	fmt.Println("8. Cancelling a task...")

	resp, err := client.CancelTask(ctx, taskID, &masterplanner.CancelTaskRequest{
		TaskID: taskID,
		Reason: "User requested cancellation",
	})
	if err != nil {
		return err
	}

	fmt.Printf("Task %s cancelled at %s\n", resp.TaskID, resp.CancelledAt.Format(time.RFC3339))
	fmt.Println()

	return nil
}
