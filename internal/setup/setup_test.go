package setup

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeClaudePreservesExistingAndIsIdempotent(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `{
  "model": "opus",
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "/usr/local/bin/mempalace hook stop"}]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Home: home, Out: io.Discard}
	if err := mergeClaude(path, "/bin/amux", opts); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if string(root["model"]) != `"opus"` {
		t.Fatalf("model lost: %s", root["model"])
	}
	var hooks map[string][]json.RawMessage
	json.Unmarshal(root["hooks"], &hooks)
	if len(hooks["Stop"]) != 2 {
		t.Fatalf("Stop groups = %d, want 2", len(hooks["Stop"]))
	}
	if !strings.Contains(string(hooks["Stop"][0]), "mempalace") {
		t.Fatal("existing hook was reordered or lost")
	}
	for _, event := range claudeEvents {
		want := 1
		if event == "Stop" {
			want = 2
		}
		if len(hooks[event]) != want {
			t.Fatalf("%s groups = %d, want %d", event, len(hooks[event]), want)
		}
	}

	if err := mergeClaude(path, "/bin/amux", opts); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(path)
	var root2 map[string]json.RawMessage
	json.Unmarshal(data2, &root2)
	var hooks2 map[string][]json.RawMessage
	json.Unmarshal(root2["hooks"], &hooks2)
	if len(hooks2["Stop"]) != 2 {
		t.Fatalf("second run duplicated hooks: %d", len(hooks2["Stop"]))
	}

	backups, _ := filepath.Glob(path + ".amux-*.bak")
	if len(backups) != 1 {
		t.Fatalf("expected one backup, got %v", backups)
	}
}

func TestMergeClaudeCreatesMissingFile(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude", "settings.json")
	if err := mergeClaude(path, "/bin/amux", Options{Home: home, Out: io.Discard}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "/bin/amux hook claude Stop") {
		t.Fatalf("missing hook: %s", data)
	}
}

func TestSetupDryRunWritesNothing(t *testing.T) {
	home := t.TempDir()
	if err := Run(Options{Home: home, Out: io.Discard, DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "amux", "tmux.conf")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote the tmux snippet")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote claude settings")
	}
}

func TestEnsureSourceUsesTildeAndIsIdempotent(t *testing.T) {
	home := t.TempDir()
	main := filepath.Join(home, ".config", "tmux", "tmux.conf")
	if err := os.MkdirAll(filepath.Dir(main), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte("set -g mouse on\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snippet := filepath.Join(home, ".config", "amux", "tmux.conf")
	opts := Options{Home: home, Out: io.Discard}
	if err := ensureSource(main, snippet, opts); err != nil {
		t.Fatal(err)
	}
	if err := ensureSource(main, snippet, opts); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(main)
	if got := strings.Count(string(data), "source-file ~/.config/amux/tmux.conf"); got != 1 {
		t.Fatalf("source line count = %d\n%s", got, data)
	}
	if !strings.Contains(string(data), "set -g mouse on") {
		t.Fatal("existing config was clobbered")
	}
}

func TestPluginIsNotClobbered(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".config", "opencode", "plugins", "amux.js")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("// hand written\nexport const X = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePlugin(path, "/bin/amux", Options{Home: home, Out: io.Discard}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "hand written") {
		t.Fatal("hand-written plugin was overwritten")
	}
}
