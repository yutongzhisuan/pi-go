package profile

import (
	"fmt"
	"slices"
	"strings"
)

// DefaultToolsets is the default whitelist for stateless Worker runs.
var DefaultToolsets = []string{"file", "web", "todo"}

// ShellClassToolsets are toolsets excluded by default due to security concerns.
var ShellClassToolsets = []string{
	"terminal",
	"code_execution",
	"browser",
	"computer_use",
	"delegation",
	"shell",
	"bash",
}

// LocalStateToolsets are toolsets that maintain local state and should be
// stripped in stateless mode even if operator added them to the whitelist.
var LocalStateToolsets = []string{
	"memory",
	"skills",
	"session_search",
	"cron",
	"messaging",
	"persistence",
	"mcp",
}

// Profile represents an executor profile that controls which toolsets are available.
type Profile struct {
	allowed   map[string]bool
	stateless bool
}

// New creates a new executor profile with the given allowed toolsets.
// If stateless is true, local-state toolsets will be filtered out even if allowed.
func New(allowedToolsets []string, stateless bool) *Profile {
	allowed := make(map[string]bool, len(allowedToolsets))
	for _, ts := range allowedToolsets {
		allowed[ts] = true
	}
	return &Profile{
		allowed:   allowed,
		stateless: stateless,
	}
}

// Default creates a profile with the default whitelist (file, web, todo).
func Default(stateless bool) *Profile {
	return New(DefaultToolsets, stateless)
}

// Resolve computes the intersection of requested toolsets with allowed toolsets.
// Returns the list of available toolsets. If empty, no tools are available.
func (p *Profile) Resolve(requested []string) []string {
	if len(requested) == 0 {
		return nil
	}

	var available []string
	for _, ts := range requested {
		ts = strings.TrimSpace(strings.ToLower(ts))
		if ts == "" {
			continue
		}

		if !p.allowed[ts] {
			continue
		}

		if p.stateless && slices.Contains(LocalStateToolsets, ts) {
			continue
		}

		available = append(available, ts)
	}

	return available
}

// Announce returns the list of toolsets to announce via acp.toolsets.
// This is the intersection of allowed toolsets minus local-state toolsets
// if stateless mode is active.
func (p *Profile) Announce() []string {
	var toolsets []string
	for ts := range p.allowed {
		if p.stateless && slices.Contains(LocalStateToolsets, ts) {
			continue
		}
		toolsets = append(toolsets, ts)
	}
	slices.Sort(toolsets)
	return toolsets
}

// ParseAllowedToolsets parses a comma-separated list of toolset names.
func ParseAllowedToolsets(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// ValidateToolsets checks if the given toolsets are known and valid.
// Returns an error describing the first invalid toolset found.
func ValidateToolsets(toolsets []string) error {
	known := make(map[string]bool)
	for _, ts := range DefaultToolsets {
		known[ts] = true
	}
	for _, ts := range ShellClassToolsets {
		known[ts] = true
	}
	for _, ts := range LocalStateToolsets {
		known[ts] = true
	}

	for _, ts := range toolsets {
		ts = strings.TrimSpace(strings.ToLower(ts))
		if ts == "" {
			continue
		}
		if !known[ts] {
			return fmt.Errorf("unknown toolset %q", ts)
		}
	}
	return nil
}
