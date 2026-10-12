package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceFileCoversModulesAndSkipsTools(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "shared", "go.mod"), "module x/shared\n\ngo 1.25.0\n")
	writeFile(t, filepath.Join(root, "services", "order", "go.mod"), "module x/order\n\ngo 1.25.1\n")
	writeFile(t, filepath.Join(root, "tools", "gen", "go.mod"), "module x/gen\n\ngo 1.26.0\n")
	writeFile(t, filepath.Join(root, "web", "node_modules", "pkg", "go.mod"), "module x/vendored\n")

	file, err := workspaceFile(root)
	if err != nil || file == "" {
		t.Fatalf("workspaceFile = %q, %v", file, err)
	}
	data, _ := os.ReadFile(file)
	got := string(data)
	for _, want := range []string{"go 1.25.1\n", "\t../services/order\n", "\t../shared\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("go.work misses %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"tools/gen", "node_modules", "go 1.26.0"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("go.work must not contain %q:\n%s", unwanted, got)
		}
	}
}

func TestWorkspaceFileNotNeeded(t *testing.T) {
	single := t.TempDir()
	writeFile(t, filepath.Join(single, "go.mod"), "module x\n\ngo 1.25.0\n")
	if file, err := workspaceFile(single); err != nil || file != "" {
		t.Fatalf("single module: workspaceFile = %q, %v; want none", file, err)
	}

	existing := t.TempDir()
	writeFile(t, filepath.Join(existing, "a", "go.mod"), "module a\n")
	writeFile(t, filepath.Join(existing, "b", "go.mod"), "module b\n")
	writeFile(t, filepath.Join(existing, "go.work"), "go 1.25.0\n")
	if file, err := workspaceFile(existing); err != nil || file != "" {
		t.Fatalf("existing go.work: workspaceFile = %q, %v; want the repo's own go.work", file, err)
	}
}

func TestVersionLess(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{{"1.25.0", "1.25.1", true}, {"1.25", "1.25.0", false}, {"1.9", "1.25", true}, {"1.26.0", "1.25.9", false}} {
		if got := versionLess(tc.a, tc.b); got != tc.want {
			t.Errorf("versionLess(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSetEnvReplacesCaseInsensitively(t *testing.T) {
	env := setEnv([]string{"Path=x", "gowork=old", "A=1"}, "GOWORK", "new")
	joined := strings.Join(env, ";")
	if strings.Contains(joined, "old") || !strings.Contains(joined, "GOWORK=new") || !strings.Contains(joined, "A=1") {
		t.Fatalf("setEnv = %v", env)
	}
}
