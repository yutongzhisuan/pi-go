package profile

import (
	"slices"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name      string
		allowed   []string
		requested []string
		stateless bool
		want      []string
	}{
		{
			name:      "default intersection",
			allowed:   DefaultToolsets,
			requested: []string{"file", "web", "shell"},
			stateless: false,
			want:      []string{"file", "web"},
		},
		{
			name:      "empty requested",
			allowed:   DefaultToolsets,
			requested: []string{},
			stateless: false,
			want:      nil,
		},
		{
			name:      "no intersection",
			allowed:   DefaultToolsets,
			requested: []string{"shell", "bash"},
			stateless: false,
			want:      nil,
		},
		{
			name:      "stateless strips local-state",
			allowed:   []string{"file", "web", "memory", "skills"},
			requested: []string{"file", "web", "memory", "skills"},
			stateless: true,
			want:      []string{"file", "web"},
		},
		{
			name:      "case insensitive and trimmed",
			allowed:   DefaultToolsets,
			requested: []string{" FILE ", "Web", "  todo  "},
			stateless: false,
			want:      []string{"file", "web", "todo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.allowed, tt.stateless)
			got := p.Resolve(tt.requested)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnnounce(t *testing.T) {
	tests := []struct {
		name      string
		allowed   []string
		stateless bool
		want      []string
	}{
		{
			name:      "default profile",
			allowed:   DefaultToolsets,
			stateless: false,
			want:      []string{"file", "todo", "web"},
		},
		{
			name:      "stateless strips local-state",
			allowed:   []string{"file", "web", "memory", "skills"},
			stateless: true,
			want:      []string{"file", "web"},
		},
		{
			name:      "no local-state in allowed",
			allowed:   []string{"file", "web"},
			stateless: true,
			want:      []string{"file", "web"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.allowed, tt.stateless)
			got := p.Announce()
			slices.Sort(got)
			slices.Sort(tt.want)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Announce() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAllowedToolsets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "comma-separated",
			input: "file,web,todo",
			want:  []string{"file", "web", "todo"},
		},
		{
			name:  "with spaces",
			input: "file , web , todo",
			want:  []string{"file", "web", "todo"},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "single toolset",
			input: "file",
			want:  []string{"file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAllowedToolsets(tt.input)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ParseAllowedToolsets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateToolsets(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		wantErr bool
	}{
		{
			name:    "valid default toolsets",
			input:   DefaultToolsets,
			wantErr: false,
		},
		{
			name:    "valid shell-class toolsets",
			input:   ShellClassToolsets,
			wantErr: false,
		},
		{
			name:    "valid local-state toolsets",
			input:   LocalStateToolsets,
			wantErr: false,
		},
		{
			name:    "unknown toolset",
			input:   []string{"file", "unknown"},
			wantErr: true,
		},
		{
			name:    "empty list",
			input:   []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolsets(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateToolsets() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
