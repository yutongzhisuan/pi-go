package tools

import (
	"fmt"

	"google.golang.org/adk/v2/tool"

	"github.com/dimetron/pi-go/internal/masterplanner"
)

// AppendMasterPlannerTools adds gateway_* tools when master planner mode is enabled.
func AppendMasterPlannerTools(existing []tool.Tool) ([]tool.Tool, error) {
	if !masterplanner.Enabled() {
		return existing, nil
	}
	mpTools, err := masterplanner.MasterPlannerTools()
	if err != nil {
		return nil, fmt.Errorf("master planner tools: %w", err)
	}
	return append(existing, mpTools...), nil
}
