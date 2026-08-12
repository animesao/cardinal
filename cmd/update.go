//go:build linux

package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var baseURL = getBaseURL()

func getBaseURL() string {
	if m := os.Getenv("DCK_UPDATE_MIRROR"); m != "" {
		return strings.TrimRight(m, "/")
	}
	return repoURL
}

func Update(args []string) {
	checkOnly := false
	for _, a := range args {
		if a == "--check" || a == "-c" {
			checkOnly = true
		}
	}

	fmt.Printf("Current version: %s\n", version)

	latest, err := fetchLatestVersion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Latest version:  %s\n", latest)

	if compareVersions(latest, version) <= 0 {
		fmt.Println("You are already up to date.")
		return
	}

	fmt.Printf("Update available: %s → %s\n", version, latest)

	if checkOnly {
		return
	}

	fmt.Print("Download and install? [y/N] ")
	var confirm string
	if _, err := fmt.Scanln(&confirm); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading confirmation: %v\n", err)
		return
	}
	if confirm != "y" && confirm != "Y" {
		fmt.Println("Update cancelled.")
		return
	}

	// Determine architecture for binary download
	arch := runtime.GOARCH
	goos := runtime.GOOS
	binaryName := fmt.Sprintf("dck-%s-%s", goos, arch)
	if goos == "windows" {
		binaryName += ".exe"
	}

	releaseTag := "v" + latest
	checksumURL := fmt.Sprintf("%s/releases/download/%s/%s.sha256", releaseURL, releaseTag, binaryName)
	binaryURL := fmt.Sprintf("%s/releases/download/%s/%s", releaseURL, releaseTag, binaryName)

	fmt.Println("Downloading update...")
	expectedChecksum, err := fetchURL(checksumURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch update checksum: %v\n", err)
		os.Exit(1)
	}

	body, err := fetchURLBytes(binaryURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to download binary: %v\n", err)
		os.Exit(1)
	}

	expectedHex, err := parseSHA256Checksum(expectedChecksum)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid update checksum: %v\n", err)
		os.Exit(1)
	}
	hash := sha256.Sum256(body)
	actualHex := hex.EncodeToString(hash[:])
	if !strings.EqualFold(actualHex, expectedHex) {
		fmt.Fprintf(os.Stderr, "Checksum mismatch! Expected %s, got %s. Aborting update.\n", expectedHex, actualHex)
		os.Exit(1)
	}
	fmt.Println("Checksum verified.")

	// Get current binary path
	selfPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get current binary path: %v\n", err)
		os.Exit(1)
	}

	// Write new binary to temp file
	tmpFile, err := os.CreateTemp("", "dck-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(body); err != nil {
		closeErr := tmpFile.Close()
		removeErr := os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Failed to write temp file: %v", err)
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "; close failed: %v", closeErr)
		}
		if removeErr != nil {
			fmt.Fprintf(os.Stderr, "; cleanup failed: %v", removeErr)
		}
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}
	if err := tmpFile.Close(); err != nil {
		removeErr := os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Failed to close temp file: %v", err)
		if removeErr != nil {
			fmt.Fprintf(os.Stderr, "; cleanup failed: %v", removeErr)
		}
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}
	if err := os.Chmod(tmpPath, 0755); err != nil {
		removeErr := os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Failed to chmod temp file: %v", err)
		if removeErr != nil {
			fmt.Fprintf(os.Stderr, "; cleanup failed: %v", removeErr)
		}
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}

	targetPath, allowSudo := updateInstallTarget(selfPath)
	if targetPath != selfPath {
		fmt.Printf("AppImage detected; installing the updated binary to %s\n", targetPath)
	}
	if err := installUpdateBinary(tmpPath, targetPath, allowSudo); err != nil {
		removeErr := os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "Failed to install update: %v", err)
		if removeErr != nil {
			fmt.Fprintf(os.Stderr, "; cleanup failed: %v", removeErr)
		}
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}
	if err := os.Remove(tmpPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not remove temporary update file: %v\n", err)
	}

	fmt.Println("Update complete!")
}

const appImageInstallPath = "/usr/local/bin/dck"

func updateInstallTarget(selfPath string) (string, bool) {
	if strings.TrimSpace(os.Getenv("APPIMAGE")) != "" || strings.Contains(selfPath, "/.mount_") {
		return appImageInstallPath, true
	}
	return selfPath, false
}

func installUpdateBinary(sourcePath, targetPath string, allowSudo bool) error {
	args := []string{"-m", "0755", sourcePath, targetPath}
	cmd := exec.Command("install", args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	firstErr := fmt.Errorf("install failed: %v: %s", err, strings.TrimSpace(string(output)))
	if !allowSudo {
		return firstErr
	}

	// An AppImage launched from a desktop session commonly runs as an ordinary
	// user, while /usr/local/bin is root-owned. Let sudo perform only the
	// verified binary installation; the checksum was checked before this point.
	sudoArgs := append([]string{"install"}, args...)
	sudoCmd := exec.Command("sudo", sudoArgs...)
	sudoCmd.Stdin = os.Stdin
	sudoCmd.Stdout = os.Stdout
	sudoCmd.Stderr = os.Stderr
	if err := sudoCmd.Run(); err != nil {
		return fmt.Errorf("%v; sudo install failed: %w", firstErr, err)
	}
	return nil
}

func parseSHA256Checksum(contents string) (string, error) {
	fields := strings.Fields(contents)
	if len(fields) == 0 {
		return "", fmt.Errorf("checksum response is empty")
	}
	checksum := fields[0]
	if len(checksum) != sha256.Size*2 {
		return "", fmt.Errorf("expected %d hex characters, got %d", sha256.Size*2, len(checksum))
	}
	if _, err := hex.DecodeString(checksum); err != nil {
		return "", fmt.Errorf("checksum is not valid hexadecimal: %w", err)
	}
	return strings.ToLower(checksum), nil
}

// binaryDownloadTimeout is long enough for multi-megabyte release binaries over
// slow links; the default 10s client timeout is too tight for that.
const binaryDownloadTimeout = 5 * time.Minute

func fetchURLBytes(url string) ([]byte, error) {
	body, err := fetchURLGoBytes(url)
	if err == nil {
		return body, nil
	}
	goErr := err
	body, err = fetchURLWithCurlBytes(url)
	if err == nil {
		return body, nil
	}
	curlErr := err
	body, err = fetchURLWithWgetBytes(url)
	if err == nil {
		return body, nil
	}
	return nil, fmt.Errorf("all methods failed (go: %v; curl: %v; wget: %v)", goErr, curlErr, err)
}

func fetchURL(url string) (string, error) {
	body, err := fetchURLGo(url)
	if err == nil {
		return body, nil
	}
	goErr := err
	body, err = fetchURLWithCurl(url)
	if err == nil {
		return body, nil
	}
	curlErr := err
	body, err = fetchURLWithWget(url)
	if err == nil {
		return body, nil
	}
	return "", fmt.Errorf("all methods failed (go: %v; curl: %v; wget: %v)", goErr, curlErr, err)
}

func fetchURLGoBytes(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: binaryDownloadTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	return body, err
}

func fetchURLGo(url string) (string, error) {
	body, err := fetchURLGoBytes(url)
	return strings.TrimSpace(string(body)), err
}

func fetchURLWithCurlBytes(url string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("curl", "-fsSL", "--connect-timeout", "10", "--max-time", "300", url)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("curl failed: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func fetchURLWithWgetBytes(url string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("wget", "-qO-", "--timeout=20", "--tries=3", url)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("wget failed: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func fetchURLWithCurl(url string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("curl", "-sL", url)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("curl failed: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

func fetchURLWithWget(url string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("wget", "-qO-", url)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("wget failed: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

func fetchLatestVersion() (string, error) {
	// Prefer git tags (e.g. v1.20.1) over VERSION file (e.g. 1.20.0-stalbal.xxx)
	if v, err := fetchVersionViaGit(); err == nil {
		return v, nil
	}
	url := baseURL + "/main/VERSION"
	v, err := fetchURL(url)
	if err == nil {
		return v, nil
	}
	return "", fmt.Errorf("could not determine latest version: %w", err)
}

func fetchVersionViaGit() (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", "ls-remote", "--tags", "https://github.com/animesao/dck.git")
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git ls-remote failed: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	// Parse the last tag matching v*.*.*
	latest := ""
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		ref := parts[1]
		if strings.HasPrefix(ref, "refs/tags/v") {
			tag := strings.TrimPrefix(ref, "refs/tags/")
			ver := strings.TrimPrefix(tag, "v")
			if compareVersions(ver, latest) > 0 {
				latest = ver
			}
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no version tags found")
	}
	return latest, nil
}

func compareVersions(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}

	ap := strings.Split(strings.TrimLeft(a, "v"), ".")
	bp := strings.Split(strings.TrimLeft(b, "v"), ".")
	max := len(ap)
	if len(bp) > max {
		max = len(bp)
	}
	for i := 0; i < max; i++ {
		var ai, bi int
		if i < len(ap) {
			_, _ = fmt.Sscanf(ap[i], "%d", &ai)
		}
		if i < len(bp) {
			_, _ = fmt.Sscanf(bp[i], "%d", &bi)
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}
