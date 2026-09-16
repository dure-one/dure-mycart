package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newPlatformArchive builds the release archive GitHub would publish for the
// platform the tests run on: a tar.gz everywhere except Windows, where the
// updater expects a zip.
//
// files maps the path inside the archive to its content.
func newPlatformArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	if runtime.GOOS == "windows" {
		zw := zip.NewWriter(&buf)
		for name, content := range files {
			w, err := zw.Create(name)
			if err != nil {
				t.Fatalf("create zip entry %s: %v", name, err)
			}
			if _, err := w.Write([]byte(content)); err != nil {
				t.Fatalf("write zip entry %s: %v", name, err)
			}
		}
		if err := zw.Close(); err != nil {
			t.Fatalf("close zip: %v", err)
		}
		return buf.Bytes()
	}

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o755,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("write tar header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write tar entry %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

// releaseHandler serves the two endpoints Init talks to: the GitHub "latest
// release" API and the asset download. It answers whatever the updater asks for,
// on whatever platform the tests run on.
func releaseHandler(t *testing.T, archive []byte) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			_, _ = fmt.Fprintf(w, `{
				"id": 1,
				"name": "v9.9.9",
				"tag_name": "v9.9.9",
				"html_url": "https://example/v9.9.9",
				"assets": [
					{"name": "checksums.txt", "browser_download_url": "%[1]s/checksums"},
					{"name": "mycart%[2]s", "browser_download_url": "%[1]s/download"}
				]
			}`, "http://"+r.Host, archiveSuffix(runtime.GOOS, runtime.GOARCH))
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(archive)
	}
}

// Init refuses to install a release whose archive does not contain the
// executable it is meant to replace. It must give up before touching the running
// binary, which is what makes this testable in-process.
func TestInit_ExtractedArchiveHasNoExecutable(t *testing.T) {
	suffix := archiveSuffix(runtime.GOOS, runtime.GOARCH)
	if suffix == "" {
		t.Skipf("no release archive is published for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	dir := t.TempDir()
	t.Chdir(dir)

	archive := newPlatformArchive(t, map[string]string{
		"README.md": "not an executable",
	})

	srv := httptest.NewServer(releaseHandler(t, archive))
	t.Cleanup(srv.Close)
	installFakeGitHub(t, srv)
	installFakeDownload(t, srv)

	err := Init(&Config{
		Owner:             "o",
		Repo:              "r",
		CurrentVersion:    "v0.0.1",
		ArchiveExecutable: "mycart",
	})
	if err == nil {
		t.Fatal("an archive without the executable was accepted")
	}
	if !strings.Contains(err.Error(), "missing or it is inaccessible") {
		t.Errorf("error %q does not explain what was wrong with the archive", err)
	}

	// The temporary release directory is cleaned up even on the failure path.
	if _, statErr := os.Stat(".lc_temp_to_delete"); !os.IsNotExist(statErr) {
		t.Errorf(".lc_temp_to_delete was left behind: %v", statErr)
	}
}

// When the running version is already the latest there is nothing to download,
// and Init says so and returns.
func TestInit_AlreadyLatest(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"name":"v1.0.0","tag_name":"v1.0.0","assets":[]}`))
	}))
	t.Cleanup(srv.Close)
	installFakeGitHub(t, srv)

	out := captureStdout(t, func() {
		if err := Init(&Config{Owner: "o", Repo: "r", CurrentVersion: "v1.0.0"}); err != nil {
			t.Errorf("Init: %v", err)
		}
	})

	if !strings.Contains(out, "You already have the latest mycart v1.0.0") {
		t.Errorf("Init did not report being up to date, output: %q", out)
	}
	if _, err := os.Stat(".lc_temp_to_delete"); !os.IsNotExist(err) {
		t.Errorf("a download directory was created even though there was nothing to download: %v", err)
	}
}

// The updater replaces the executable it is running from. That cannot be
// observed inside the test process — the file would have to be the test binary
// itself — so the child runs a *copy* of the test binary and performs the swap
// on that copy.
const (
	updateInitChildEnv = "MYCART_TEST_UPDATE_INIT_CHILD"
	updateInitURLEnv   = "MYCART_TEST_UPDATE_INIT_URL"
	updateInitExecEnv  = "MYCART_TEST_UPDATE_INIT_EXEC"
)

// newRewrittenClient returns a client that resolves every request onto base.
func newRewrittenClient(base string) *http.Client {
	return &http.Client{
		Transport: &rewriteTransport{
			base: http.DefaultTransport,
			rewrite: func(req *http.Request) *http.Request {
				u, _ := req.URL.Parse(base + req.URL.Path)
				clone := req.Clone(req.Context())
				clone.URL = u
				clone.Host = ""
				return clone
			},
		},
	}
}

func TestUpdateInitHelperProcess(t *testing.T) {
	if os.Getenv(updateInitChildEnv) == "" {
		t.Skip("only runs as the child process of TestInit_ReplacesTheExecutable")
	}

	base := os.Getenv(updateInitURLEnv)
	releaseClient = newRewrittenClient(base)
	downloadClient = newRewrittenClient(base)

	err := Init(&Config{
		Owner:             "o",
		Repo:              "r",
		CurrentVersion:    "v0.0.1",
		ArchiveExecutable: os.Getenv(updateInitExecEnv),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Init: %v\n", err)
		os.Exit(5)
	}
}

func TestInit_ReplacesTheExecutable(t *testing.T) {
	suffix := archiveSuffix(runtime.GOOS, runtime.GOARCH)
	if suffix == "" {
		t.Skipf("no release archive is published for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	const (
		execName      = "mycart"
		newExecBody   = "FAKE NEW BINARY\n"
		oldExecSuffix = ".old"
	)

	dir := t.TempDir()

	// A copy of this test binary stands in for the installed myCart build.
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("locate the test binary: %v", err)
	}
	binary := filepath.Join(dir, execName)
	copyFile(t, self, binary)

	archive := newPlatformArchive(t, map[string]string{
		execName:    newExecBody,
		"README.md": "docs",
	})

	srv := httptest.NewServer(releaseHandler(t, archive))
	t.Cleanup(srv.Close)

	cmd := exec.Command(binary, "-test.run=TestUpdateInitHelperProcess")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		updateInitChildEnv+"=1",
		updateInitURLEnv+"="+srv.URL,
		updateInitExecEnv+"="+execName,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the updater failed: %v\n%s", err, out)
	}

	for _, want := range []string{"Downloading", "Extracting", "Replacing the executable", "Update completed successfully"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the updater never reported %q, output:\n%s", want, out)
		}
	}

	// The file the updater was running from now holds the new build.
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatalf("read the replaced executable: %v", err)
	}
	if string(got) != newExecBody {
		t.Errorf("the executable was not replaced: content is %q, want %q", got, newExecBody)
	}

	// The backup and the working directory are both removed on the way out.
	if _, err := os.Stat(binary + oldExecSuffix); !os.IsNotExist(err) {
		t.Errorf("the backup %s%s was left behind: %v", binary, oldExecSuffix, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".lc_temp_to_delete")); !os.IsNotExist(err) {
		t.Errorf(".lc_temp_to_delete was left behind: %v", err)
	}
}

// captureStdout runs fn with os.Stdout redirected into a pipe and returns what
// was written: the updater reports progress on standard output, which is how a
// user watching the upgrade sees it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	prev := os.Stdout
	os.Stdout = w
	// Restoring in Cleanup keeps a panicking fn from leaving stdout swapped for
	// the rest of the test binary; the assignment below brings stdout back before
	// the caller's own output.
	t.Cleanup(func() { os.Stdout = prev })

	fn()

	os.Stdout = prev
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured output: %v", err)
	}
	_ = r.Close()

	return string(out)
}

// copyFile copies src to dst, keeping the executable bit: the updater renames
// and runs nothing, but a binary without it is not a faithful stand-in.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()

	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("open %s: %v", src, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatalf("create %s: %v", dst, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

// An installation whose binary sits in a directory the user cannot write to —
// /usr/local/bin owned by root, say — must fail with the rename error rather
// than half-update and leave no executable behind. The child runs from a
// read-only directory to reproduce that.
func TestInit_CannotReplaceTheExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the read-only-directory check is a Unix permission test")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	const execName = "mycart"

	// The running binary lives here, and the directory is made read-only after
	// the copy. The child's working directory is a different one, so the
	// download and extraction still succeed.
	binDir := t.TempDir()
	t.Cleanup(func() { _ = os.Chmod(binDir, 0o755) })

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("locate the test binary: %v", err)
	}
	binary := filepath.Join(binDir, execName)
	copyFile(t, self, binary)

	original, err := os.ReadFile(binary)
	if err != nil {
		t.Fatalf("read the stand-in executable: %v", err)
	}
	if err := os.Chmod(binDir, 0o555); err != nil {
		t.Fatalf("make the binary directory read-only: %v", err)
	}

	archive := newPlatformArchive(t, map[string]string{execName: "FAKE NEW BINARY\n"})
	srv := httptest.NewServer(releaseHandler(t, archive))
	t.Cleanup(srv.Close)

	workDir := t.TempDir()
	cmd := exec.Command(binary, "-test.run=TestUpdateInitHelperProcess")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		updateInitChildEnv+"=1",
		updateInitURLEnv+"="+srv.URL,
		updateInitExecEnv+"="+execName,
	)
	out, err := cmd.CombinedOutput()

	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("the updater did not fail on an unwritable installation: err=%v, output:\n%s", err, out)
	}
	if exit.ExitCode() != 5 {
		t.Errorf("exit code = %d, want 5; output:\n%s", exit.ExitCode(), out)
	}

	// The binary is untouched, and no half-finished backup is left next to it.
	got, err := os.ReadFile(binary)
	if err != nil {
		t.Fatalf("read the executable after the failed update: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Error("the executable was modified by an update that failed")
	}
	if _, err := os.Stat(binary + ".old"); !os.IsNotExist(err) {
		t.Errorf("a backup %s.old was left behind: %v", binary, err)
	}
}

// ReleaseInfo hands back the asset built for this platform, skipping the ones
// that are not.
func TestReleaseInfo_PicksTheAssetForThisPlatform(t *testing.T) {
	suffix := archiveSuffix(runtime.GOOS, runtime.GOARCH)
	if suffix == "" {
		t.Skipf("no release archive is published for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{
			"name": "v9.9.9",
			"tag_name": "v9.9.9",
			"assets": [
				{"name": "checksums.txt", "browser_download_url": "https://example/checksums"},
				{"name": "mycart%s", "browser_download_url": "https://example/mycart"}
			]
		}`, suffix)
	}))
	t.Cleanup(srv.Close)
	installFakeGitHub(t, srv)

	asset, err := ReleaseInfo(context.Background(), &Config{Owner: "o", Repo: "r", CurrentVersion: "v0.0.1"})
	if err != nil {
		t.Fatalf("ReleaseInfo: %v", err)
	}
	if asset == nil {
		t.Fatal("ReleaseInfo returned no asset for a newer release")
	}
	if want := "mycart" + suffix; asset.Name != want {
		t.Errorf("asset = %q, want %q", asset.Name, want)
	}
	if asset.DownloadUrl != "https://example/mycart" {
		t.Errorf("download url = %q", asset.DownloadUrl)
	}
}

// A newer release with no build for this platform must fail, not silently
// download somebody else's binary.
func TestReleaseInfo_MissingAssetIsAnError(t *testing.T) {
	suffix := archiveSuffix(runtime.GOOS, runtime.GOARCH)
	if suffix == "" {
		t.Skipf("no release archive is published for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"name": "v9.9.9",
			"tag_name": "v9.9.9",
			"assets": [{"name": "checksums.txt", "browser_download_url": "https://example/checksums"}]
		}`))
	}))
	t.Cleanup(srv.Close)
	installFakeGitHub(t, srv)

	asset, err := ReleaseInfo(context.Background(), &Config{Owner: "o", Repo: "r", CurrentVersion: "v0.0.1"})
	if err == nil {
		t.Fatalf("ReleaseInfo returned %+v for a release with no matching asset", asset)
	}
	if !strings.Contains(err.Error(), "missing asset containing") {
		t.Errorf("error %q does not explain that the asset is missing", err)
	}

	// An unknown platform has no suffix at all, and must be refused the same way.
	if got := archiveSuffix(runtime.GOOS, "mips"); got != "" {
		t.Errorf("archiveSuffix(%s, mips) = %q, want empty", runtime.GOOS, got)
	}
}
