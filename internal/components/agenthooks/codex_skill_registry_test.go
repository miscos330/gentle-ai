package agenthooks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v5/internal/agents"
	"github.com/gentleman-programming/gentle-ai/v5/internal/model"
)

func TestCodexSkillRegistryCommandForWindows(t *testing.T) {
	const want = `powershell -NoProfile -Command 'gentle-ai skill-registry refresh --quiet --no-gitignore --cwd "$PWD"; exit 0'`
	if got := codexSkillRegistryCommand("windows"); got != want {
		t.Fatalf("Windows hook command = %q, want %q", got, want)
	}
	if got := codexSkillRegistryCommand("linux"); got != codexLegacySkillRegistryCommand {
		t.Fatalf("Linux hook command = %q, want %q", got, codexLegacySkillRegistryCommand)
	}
}

// A Windows hooks.json written before the PowerShell form keeps only the new
// command after a sync, beside the user's own SessionStart hooks.
func TestCodexSkillRegistryWindowsMigration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", "")
	adapter, err := agents.NewAdapter(model.AgentCodex)
	if err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(adapter.GlobalConfigDir(home), "hooks.json")
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{"hooks":{"SessionStart":[` +
		`{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":` + mustJSON(t, codexLegacySkillRegistryCommand) + `,"timeout":30}]},` +
		`{"matcher":"","hooks":[{"type":"command","command":"echo keep"}]}]}}`
	if err := os.WriteFile(hooksPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := installSkillRegistry(home, adapter, "windows")
	if err != nil || !result.Changed {
		t.Fatalf("first Windows install: %+v %v", result, err)
	}
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	var commands []string
	for _, entry := range root.Hooks["SessionStart"] {
		for _, hook := range entry.Hooks {
			commands = append(commands, hook.Command)
		}
	}
	joined := strings.Join(commands, "\n")
	canonical := codexSkillRegistryCommand("windows")
	if strings.Count(joined, canonical) != 1 || strings.Contains(joined, codexLegacySkillRegistryCommand) || !strings.Contains(joined, "echo keep") {
		t.Fatalf("after migration SessionStart commands = %q", commands)
	}

	second, err := installSkillRegistry(home, adapter, "windows")
	if err != nil || second.Changed {
		t.Fatalf("second Windows install: %+v %v", second, err)
	}
	after, err := os.ReadFile(hooksPath)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatalf("migration not byte-idempotent: %v", err)
	}
}
