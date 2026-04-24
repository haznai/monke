package appdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyDataCopiesIntoSafeRoot(t *testing.T) {
	t.Setenv("MONKEYTYPE_TUI_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	legacyRoot := filepath.Join(home, ".monkeytype-tui")
	legacyDatasets := filepath.Join(legacyRoot, "datasets")
	if err := os.MkdirAll(legacyDatasets, 0o755); err != nil {
		t.Fatalf("MkdirAll legacy datasets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyRoot, "history.json"), []byte(`{"tests":[]}`), 0o644); err != nil {
		t.Fatalf("WriteFile history: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyRoot, ".env"), []byte("GROQ_API_KEY=legacy-key\n"), 0o600); err != nil {
		t.Fatalf("WriteFile env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDatasets, "english.json"), []byte(`{"name":"english","words":["one"]}`), 0o644); err != nil {
		t.Fatalf("WriteFile dataset: %v", err)
	}

	cwd := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
	if err := os.MkdirAll(filepath.Join(cwd, ".monkeytype-tui"), 0o755); err != nil {
		t.Fatalf("MkdirAll local llm dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".monkeytype-tui", "llm-calls.sqlite3"), []byte("sqlite-bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile llm log: %v", err)
	}

	if err := MigrateLegacyData(); err != nil {
		t.Fatalf("MigrateLegacyData: %v", err)
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	root := filepath.Join(configDir, "monkeytype-tui")

	assertFileContent(t, filepath.Join(root, "history.json"), `{"tests":[]}`)
	assertFileContent(t, filepath.Join(root, ".env"), "GROQ_API_KEY=legacy-key\n")
	assertFileContent(t, filepath.Join(root, "datasets", "english.json"), `{"name":"english","words":["one"]}`)
	assertFileContent(t, filepath.Join(root, "llm-calls.sqlite3"), "sqlite-bytes")
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("file %s = %q, want %q", path, string(data), want)
	}
}
