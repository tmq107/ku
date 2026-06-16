// Package upgrade implements self-upgrade by downloading the latest release
// binary from GitHub.
package upgrade

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repo   = "bjarneo/kli" // the legacy binary and repo
	kuRepo = "bjarneo/ku"  // the project's new home after the rename
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type release struct {
	TagName string `json:"tag_name"`
}

// Run checks for a newer GitHub release and replaces the current binary. It
// also checks the renamed "ku" repo: when ku carries the newest release, it
// points the user there instead of upgrading the old kli binary.
func Run(currentVersion string) error {
	kliLatest, kliErr := latestVersionFor(repo)
	kuLatest, kuErr := latestVersionFor(kuRepo)

	if migrateToKu(kuLatest, kuErr, kliLatest, kliErr) {
		return announceRename(kuLatest)
	}
	if kliErr != nil {
		return fmt.Errorf("checking latest version: %w", kliErr)
	}
	latest := kliLatest

	currentVersion = strings.TrimSpace(currentVersion)
	if currentVersion != "" && currentVersion != "dev" && currentVersion == latest {
		fmt.Printf("Already up to date (%s)\n", currentVersion)
		return nil
	}

	if currentVersion == "" || currentVersion == "dev" {
		fmt.Printf("Latest release is %s, downloading...\n", latest)
	} else {
		fmt.Printf("Upgrading %s -> %s\n", currentVersion, latest)
	}

	asset, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", repo, asset)
	checksumURL := fmt.Sprintf("https://github.com/%s/releases/latest/download/checksums.txt", repo)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating current binary: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}

	checksum, err := latestChecksum(checksumURL, asset)
	if err != nil {
		return fmt.Errorf("checking checksum: %w", err)
	}
	if err := downloadAndReplace(url, exe, checksum); err != nil {
		return err
	}

	fmt.Printf("Upgraded to %s\n", latest)
	return nil
}

func assetName(goos, goarch string) (string, error) {
	switch goos {
	case "linux", "darwin", "windows":
	default:
		return "", fmt.Errorf("unsupported OS: %s", goos)
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture: %s", goarch)
	}
	name := fmt.Sprintf("kli-%s-%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name, nil
}

func latestVersion() (string, error) { return latestVersionFor(repo) }

func latestVersionFor(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var r release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}
	if r.TagName == "" {
		return "", fmt.Errorf("release has no tag_name")
	}
	return r.TagName, nil
}

// migrateToKu reports whether ku now carries the newest release (so the user
// should move there). It is true when ku has a release and is at least as new
// as kli, or kli has no resolvable release while ku does.
func migrateToKu(kuLatest string, kuErr error, kliLatest string, kliErr error) bool {
	if kuErr != nil {
		return false
	}
	return kliErr != nil || compareVersions(kuLatest, kliLatest) >= 0
}

// compareVersions orders two "vMAJOR.MINOR.PATCH" tags: -1 if a < b, 0 if
// equal, 1 if a > b. Any pre-release/build suffix is ignored.
func compareVersions(a, b string) int {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := range pa {
		switch {
		case pa[i] < pb[i]:
			return -1
		case pa[i] > pb[i]:
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		out[i], _ = strconv.Atoi(part)
	}
	return out
}

// announceRename tells the user the project is now "ku" and prompts them to
// install it and delete the old kli binary.
func announceRename(kuLatest string) error {
	exe, err := os.Executable()
	if err == nil {
		if resolved, rerr := filepath.EvalSymlinks(exe); rerr == nil {
			exe = resolved
		}
	} else {
		exe = ""
	}
	fmt.Print(renameNotice(kuLatest, exe))
	return nil
}

func renameNotice(kuLatest, exe string) string {
	var b strings.Builder
	fmt.Fprint(&b, "kli has been renamed to ku.\n\n")
	fmt.Fprintf(&b, "The newest version (%s) is now published as \"ku\":\n", kuLatest)
	fmt.Fprintf(&b, "  https://github.com/%s\n\n", kuRepo)
	fmt.Fprint(&b, "Install ku, then delete this old kli binary:\n")
	fmt.Fprintf(&b, "  curl -fsSL https://raw.githubusercontent.com/%s/main/install.sh | sh\n", kuRepo)
	if exe != "" {
		fmt.Fprintf(&b, "  rm %s\n", exe)
	}
	return b.String()
}

func latestChecksum(url, asset string) (string, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found", asset)
}

func downloadAndReplace(url, destPath, checksum string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	dir := filepath.Dir(destPath)
	tmp, err := os.CreateTemp(dir, "kli-upgrade-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w (try running with sudo)", err)
	}
	tmpPath := tmp.Name()

	const maxBinarySize = 200 << 20
	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, maxBinarySize)); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("writing binary: %w", err)
	}

	if err := verifyFileChecksum(tmpPath, checksum); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replacing binary: %w (try running with sudo)", err)
	}
	return nil
}
