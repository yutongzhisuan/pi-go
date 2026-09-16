package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckModel_NoModelRequested(t *testing.T) {
	cfg := Config{}
	err := cfg.CheckModel(context.Background(), "")
	if err != nil {
		t.Errorf("CheckModel() with empty model should return nil, got: %v", err)
	}
}

func TestCheckModel_AllowedList(t *testing.T) {
	cfg := Config{
		AllowedModels: []string{"model-a", "model-b"},
	}

	err := cfg.CheckModel(context.Background(), "model-c")
	if err == nil {
		t.Error("CheckModel() should fail for model not in allowed list")
	}
}

func TestCheckModel_RuntimeAvailability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		resp := map[string]interface{}{
			"data": []map[string]string{
				{"id": "model-a"},
				{"id": "model-b"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL: server.URL,
	}

	err := cfg.CheckModel(context.Background(), "model-a")
	if err != nil {
		t.Errorf("CheckModel() should succeed for available model: %v", err)
	}

	err = cfg.CheckModel(context.Background(), "model-c")
	if err == nil {
		t.Error("CheckModel() should fail for unavailable model")
	}
}

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		resp := map[string]interface{}{
			"data": []map[string]string{
				{"id": "model-a"},
				{"id": "model-b"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL: server.URL,
	}

	models, err := cfg.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() failed: %v", err)
	}

	expected := []string{"model-a", "model-b"}
	if len(models) != len(expected) {
		t.Errorf("ListModels() returned %d models, want %d", len(models), len(expected))
	}

	for i, want := range expected {
		if i >= len(models) || models[i] != want {
			t.Errorf("ListModels()[%d] = %q, want %q", i, models[i], want)
		}
	}
}
