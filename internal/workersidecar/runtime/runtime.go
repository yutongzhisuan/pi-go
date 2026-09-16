package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config holds configuration for local runtime model checking.
type Config struct {
	BaseURL       string
	APIKey        string
	AllowedModels []string
}

// DefaultConfig returns the default runtime configuration from environment.
func DefaultConfig() Config {
	baseURL := "http://127.0.0.1:8080/v1"
	if env := os.Getenv("ACP_LOCAL_RUNTIME_BASE_URL"); env != "" {
		baseURL = env
	}

	apiKey := "no-key-required"
	if env := os.Getenv("ACP_LOCAL_RUNTIME_API_KEY"); env != "" {
		apiKey = env
	}

	var allowedModels []string
	if env := os.Getenv("ACP_ALLOWED_MODELS"); env != "" {
		for _, m := range strings.Split(env, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				allowedModels = append(allowedModels, m)
			}
		}
	}

	return Config{
		BaseURL:       baseURL,
		APIKey:        apiKey,
		AllowedModels: allowedModels,
	}
}

// CheckModel verifies that the requested model is available in the local runtime.
// Returns nil if the model is available, or an error describing the failure.
func (c Config) CheckModel(ctx context.Context, modelName string) error {
	if modelName == "" {
		return nil
	}

	if len(c.AllowedModels) > 0 {
		found := false
		for _, allowed := range c.AllowedModels {
			if allowed == modelName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("model %q not in allowed list: %v", modelName, c.AllowedModels)
		}
	}

	models, err := c.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to list models from runtime: %w", err)
	}

	for _, m := range models {
		if m == modelName {
			return nil
		}
	}

	return fmt.Errorf("model %q not available in runtime (available: %v)", modelName, models)
}

// ListModels queries the local runtime for available models.
func (c Config) ListModels(ctx context.Context) ([]string, error) {
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/models"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("runtime returned status %d", resp.StatusCode)
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID != "" {
			models = append(models, item.ID)
		}
	}

	return models, nil
}

