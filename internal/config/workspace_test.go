package config

import (
	"strings"
	"testing"
)

func TestValidateProjectRejectsEscapingWorkspaceGlob(t *testing.T) {
	for _, glob := range []string{"../outside", "..", "/abs/path", "c:/win/path"} {
		project := NewProject(t.TempDir())
		project.Workspaces = []string{glob}
		if err := ValidateProject(project); err == nil {
			t.Fatalf("workspace glob %q should be rejected", glob)
		}
	}
}

func TestValidateProjectAcceptsWorkspaceGlobs(t *testing.T) {
	project := NewProject(t.TempDir())
	project.Workspaces = []string{"packages/*", "libs/*", "./modules/*", "packages/*", ""}
	if err := ValidateProject(project); err != nil {
		t.Fatal(err)
	}
	// 归一化：去掉 ./ 前缀、去重、忽略空项。
	want := []string{"packages/*", "libs/*", "modules/*"}
	if strings.Join(project.Workspaces, ",") != strings.Join(want, ",") {
		t.Fatalf("workspaces = %#v, want %#v", project.Workspaces, want)
	}
}

func TestWorkspaceGlobRoundTrip(t *testing.T) {
	dir := t.TempDir()
	project := NewProject(dir)
	project.Workspaces = []string{"components/*"}
	if err := SaveProject(dir, project); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Workspaces) != 1 || loaded.Workspaces[0] != "components/*" {
		t.Fatalf("workspaces = %#v", loaded.Workspaces)
	}
}

func TestNewProjectLeavesWorkspacesEmpty(t *testing.T) {
	project := NewProject(t.TempDir())
	if len(project.Workspaces) != 0 {
		t.Fatalf("workspaces = %#v, want empty (default applies at read time)", project.Workspaces)
	}
}
