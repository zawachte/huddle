package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach/huddle/internal/model"
)

func TestFilePlanAndApply(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "example.conf")
	if err := os.WriteFile(path, []byte("old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	content := "new\n"
	cfg := model.Config{Resources: []model.Resource{{ID: "example", Type: "file", Path: path, Content: &content, Mode: "0644"}}}
	plan, err := BuildPlan("test.yaml", cfg, []byte("config"))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 1 || !strings.Contains(plan.Changes[0].Diff, "-old") || !strings.Contains(plan.Changes[0].Diff, "+new") {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(got) != content || info.Mode().Perm() != 0644 {
		t.Fatalf("got content=%q mode=%o", got, info.Mode().Perm())
	}
}

func TestApplyRejectsStaleFilePlan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "example")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	content := "new"
	plan, err := BuildPlan("test.yaml", model.Config{Resources: []model.Resource{{ID: "example", Type: "file", Path: path, Content: &content}}}, []byte("config"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("someone else changed this"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan); err == nil || !strings.Contains(err.Error(), "stale plan") {
		t.Fatalf("expected stale plan error, got %v", err)
	}
}
