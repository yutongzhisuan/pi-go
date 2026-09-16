package tools

import (
	"strings"

	"google.golang.org/adk/v2/tool"
	"google.golang.org/genai"

	"github.com/dimetron/pi-go/internal/workersidecar/profile"
)

// toolNamesForExecutorToolsets maps Hermes executor toolset names to pi-go tool names.
var toolNamesForExecutorToolsets = map[string][]string{
	"file": {"read", "write", "edit", "grep", "find", "ls", "tree", "read_image", "git-overview", "git-file-diff", "git-hunk"},
	"web":  {"fetch_docs"},
	"todo": {},
	"terminal": {"bash", "bash_wait", "bash_kill"},
	"shell":    {"bash", "bash_wait", "bash_kill"},
	"code_execution": {"bash", "bash_wait", "bash_kill"},
}

// FilterByExecutorToolsets keeps only tools whose names appear in the resolved toolset list.
// When toolsets is empty, returns an empty slice (Hermes intersection semantics).
func FilterByExecutorToolsets(all []tool.Tool, toolsets []string) []tool.Tool {
	if len(toolsets) == 0 {
		return nil
	}
	allowed := make(map[string]bool)
	for _, ts := range toolsets {
		ts = strings.TrimSpace(strings.ToLower(ts))
		for _, name := range toolNamesForExecutorToolsets[ts] {
			allowed[name] = true
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	out := make([]tool.Tool, 0, len(all))
	for _, t := range all {
		if name := toolDeclarationName(t); name != "" && allowed[name] {
			out = append(out, t)
		}
	}
	return out
}

// ParseResolvedExecutorToolsets parses PI_ACP_RESOLVED_TOOLSETS (comma-separated Hermes toolset names).
func ParseResolvedExecutorToolsets(raw string) []string {
	return profile.ParseAllowedToolsets(raw)
}

func toolDeclarationName(t tool.Tool) string {
	type declarer interface {
		Declaration() *genai.FunctionDeclaration
	}
	d, ok := t.(declarer)
	if !ok || d.Declaration() == nil {
		return ""
	}
	return d.Declaration().Name
}
