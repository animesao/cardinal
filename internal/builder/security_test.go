package builder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeBuildSourceRejectsSymlinkEscape(t *testing.T) {
	contextDir := t.TempDir()
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(contextDir, "secret-link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := safeBuildSource(contextDir, "secret-link"); err == nil {
		t.Fatal("safeBuildSource accepted a symlink escaping the build context")
	}
}

func TestCopyRecursiveRejectsNestedSymlink(t *testing.T) {
	src := t.TempDir()
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "secret")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(src, "nested-link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := copyRecursive(src, filepath.Join(t.TempDir(), "copy")); err == nil {
		t.Fatal("copyRecursive followed a nested symlink")
	}
}

func TestCopyRecursiveRejectsSymlinkDestinationDirectory(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "file"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	dstRoot := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dstRoot, "dst")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := copyRecursive(src, filepath.Join(dstRoot, "dst")); err == nil {
		t.Fatal("copyRecursive accepted a symlink destination directory")
	}
}

func TestSafeBuildDestinationRejectsSymlinkAncestor(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := safeBuildDestination(root, "/escape/file"); err == nil {
		t.Fatal("safeBuildDestination accepted a symlink ancestor")
	}
}

func TestBuildEnvironmentDoesNotReadHostEnvironment(t *testing.T) {
	const key = "CARDINAL_SECURITY_TEST_HOST_SECRET"
	if err := os.Setenv(key, "must-not-leak"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv(key) })
	args := substituteArgs([]string{"${" + key + "}"}, map[string]string{})
	if len(args) != 1 || args[0] != "" {
		t.Fatalf("substituteArgs leaked host environment: %#v", args)
	}
}
