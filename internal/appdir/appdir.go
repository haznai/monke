package appdir

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	appName           = "monkeytype-tui"
	legacyDirName     = ".monkeytype-tui"
	rootOverrideEnv   = "MONKEYTYPE_TUI_HOME"
	llmLogOverrideEnv = "MONKEYTYPE_TUI_LLM_LOG_PATH"
	llmLogFileName    = "llm-calls.sqlite3"
	historyFileName   = "history.json"
	datasetsDirName   = "datasets"
	envFileName       = ".env"
)

func Root() string {
	if override := strings.TrimSpace(os.Getenv(rootOverrideEnv)); override != "" {
		return filepath.Clean(override)
	}
	if configDir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(configDir, appName)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, legacyDirName)
	}
	return legacyDirName
}

func DatasetDir() string {
	return filepath.Join(Root(), datasetsDirName)
}

func HistoryPath() string {
	return filepath.Join(Root(), historyFileName)
}

func LLMLogPath() string {
	if override := strings.TrimSpace(os.Getenv(llmLogOverrideEnv)); override != "" {
		return filepath.Clean(override)
	}
	return filepath.Join(Root(), llmLogFileName)
}

func EnvPath() string {
	return filepath.Join(Root(), envFileName)
}

func legacyRoot() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, legacyDirName)
	}
	return legacyDirName
}

func legacyDatasetDir() string {
	return filepath.Join(legacyRoot(), datasetsDirName)
}

func legacyHistoryPath() string {
	return filepath.Join(legacyRoot(), historyFileName)
}

func legacyEnvPath() string {
	return filepath.Join(legacyRoot(), envFileName)
}

func localLegacyLLMLogPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, legacyDirName, llmLogFileName), nil
}

func MigrateLegacyData() error {
	if strings.TrimSpace(os.Getenv(rootOverrideEnv)) != "" {
		return nil
	}

	if err := copyFileIfMissing(HistoryPath(), legacyHistoryPath()); err != nil {
		return fmt.Errorf("migrate history: %w", err)
	}
	if err := copyDirIfMissing(DatasetDir(), legacyDatasetDir()); err != nil {
		return fmt.Errorf("migrate datasets: %w", err)
	}
	if err := copyFileIfMissing(EnvPath(), legacyEnvPath()); err != nil {
		return fmt.Errorf("migrate env: %w", err)
	}
	localLogPath, err := localLegacyLLMLogPath()
	if err != nil {
		return nil
	}
	if err := copySQLiteBundleIfMissing(LLMLogPath(), localLogPath); err != nil {
		return fmt.Errorf("migrate llm log: %w", err)
	}
	return nil
}

func copyDirIfMissing(dest, src string) error {
	if samePath(dest, src) {
		return nil
	}
	if _, err := os.Stat(dest); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(path, target)
	})
}

func copySQLiteBundleIfMissing(dest, src string) error {
	if samePath(dest, src) {
		return nil
	}
	if _, err := os.Stat(dest); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := copyFileIfMissing(dest, src); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := copyFileIfMissing(dest+suffix, src+suffix); err != nil {
			return err
		}
	}
	return nil
}

func copyFileIfMissing(dest, src string) error {
	if samePath(dest, src) {
		return nil
	}
	if _, err := os.Stat(dest); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return copyFile(src, dest)
}

func copyFile(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
