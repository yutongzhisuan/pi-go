package tools

import "testing"

func TestFilterByExecutorToolsetsFileOnly(t *testing.T) {
	sb, err := NewSandbox(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	defer sb.Close()
	core, err := CoreTools(sb, WithBashSupervisor(NewBashSupervisor()))
	if err != nil {
		t.Fatalf("CoreTools: %v", err)
	}
	filtered := FilterByExecutorToolsets(core, []string{"file"})
	if len(filtered) == 0 {
		t.Fatal("expected file tools")
	}
	for _, tool := range filtered {
		name := toolDeclarationName(tool)
		if name == "bash" {
			t.Fatal("bash should be excluded for file-only toolsets")
		}
	}
}
