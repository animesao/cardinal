//go:build linux

package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateInstallTarget(t *testing.T) {
	tests := []struct {
		name       string
		selfPath   string
		appImage   string
		wantTarget string
		wantSudo   bool
	}{
		{
			name:       "normal binary",
			selfPath:   "/usr/local/bin/dck",
			wantTarget: "/usr/local/bin/dck",
		},
		{
			name:       "mounted AppImage executable",
			selfPath:   "/tmp/.mount_dckabc/usr/bin/dck",
			wantTarget: appImageInstallPath,
			wantSudo:   true,
		},
		{
			name:       "AppImage environment",
			selfPath:   "/opt/dck.AppImage",
			appImage:   "/opt/dck.AppImage",
			wantTarget: appImageInstallPath,
			wantSudo:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old, wasSet := os.LookupEnv("APPIMAGE")
			t.Cleanup(func() {
				if wasSet {
					_ = os.Setenv("APPIMAGE", old)
				} else {
					_ = os.Unsetenv("APPIMAGE")
				}
			})
			if tt.appImage == "" {
				_ = os.Unsetenv("APPIMAGE")
			} else {
				_ = os.Setenv("APPIMAGE", tt.appImage)
			}

			gotTarget, gotSudo := updateInstallTarget(tt.selfPath)
			if gotTarget != tt.wantTarget || gotSudo != tt.wantSudo {
				t.Fatalf("updateInstallTarget(%q) = (%q, %t), want (%q, %t)", tt.selfPath, gotTarget, gotSudo, tt.wantTarget, tt.wantSudo)
			}
		})
	}
}

func TestInstallUpdateBinary(t *testing.T) {
	source := filepath.Join(t.TempDir(), "dck-new")
	target := filepath.Join(t.TempDir(), "bin", "dck")
	if err := os.WriteFile(source, []byte("new binary"), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := installUpdateBinary(source, target, false); err != nil {
		t.Fatalf("installUpdateBinary: %v", err)
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(contents) != "new binary" {
		t.Fatalf("target contents = %q, want %q", contents, "new binary")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("target mode = %o, want 755", info.Mode().Perm())
	}
}
