package orchestrator

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	original, wasSet := os.LookupEnv("DCK_DATA_DIR")
	tmpDir, err := os.MkdirTemp("", "dck-orchestrator-tests-")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("DCK_DATA_DIR", tmpDir); err != nil {
		panic(err)
	}

	code := m.Run()

	if wasSet {
		_ = os.Setenv("DCK_DATA_DIR", original)
	} else {
		_ = os.Unsetenv("DCK_DATA_DIR")
	}
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}
