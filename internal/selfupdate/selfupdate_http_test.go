package selfupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/buildinfo"
)

func TestGHReleaseAsset(t *testing.T) {
	rel := ghRelease{
		TagName: "v2.0.0",
		Assets: []ghAsset{
			{Name: "screpdb-linux-amd64", URL: "https://example.test/a"},
			{Name: "SHA256SUMS", URL: "https://example.test/s"},
		},
	}
	cases := []struct {
		name    string
		wantOK  bool
		wantURL string
	}{
		{"screpdb-linux-amd64", true, "https://example.test/a"},
		{"SHA256SUMS", true, "https://example.test/s"},
		{"screpdb-windows-amd64.exe", false, ""},
		{"", false, ""},
	}
	for _, c := range cases {
		a, ok := rel.asset(c.name)
		if ok != c.wantOK || a.URL != c.wantURL {
			t.Errorf("asset(%q) = (%+v,%v), want URL %q ok %v", c.name, a, ok, c.wantURL, c.wantOK)
		}
	}
}

func TestDownloadAssetSuccess(t *testing.T) {
	body := []byte("binary payload")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	got, err := downloadAsset(context.Background(), ghAsset{Name: "bin", URL: srv.URL})
	if err != nil {
		t.Fatalf("downloadAsset: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("got %q, want %q", got, body)
	}
}

func TestDownloadAssetNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := downloadAsset(context.Background(), ghAsset{Name: "bin", URL: srv.URL}); err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestDownloadAssetBadURL(t *testing.T) {
	if _, err := downloadAsset(context.Background(), ghAsset{Name: "bin", URL: "http://127.0.0.1:0/x"}); err == nil {
		t.Fatal("expected error dialing an invalid port")
	}
}

func TestDetectInstall(t *testing.T) {
	origVersion := buildinfo.Version
	defer func() { buildinfo.Version = origVersion }()

	t.Run("dev build", func(t *testing.T) {
		buildinfo.Version = "dev"
		if inst := detectInstall(filepath.Join(t.TempDir(), "screpdb")); inst.supported || inst.reason != ReasonDevBuild {
			t.Errorf("got %+v, want unsupported dev-build", inst)
		}
	})

	t.Run("managed", func(t *testing.T) {
		buildinfo.Version = "v1.2.3"
		self := "/opt/homebrew/Cellar/screpdb/1.2.3/bin/screpdb"
		inst := detectInstall(self)
		if inst.supported || inst.reason != ReasonManaged || inst.manager != "homebrew" {
			t.Errorf("got %+v, want unsupported managed/homebrew", inst)
		}
	})

	t.Run("not writable", func(t *testing.T) {
		buildinfo.Version = "v1.2.3"
		self := filepath.Join(t.TempDir(), "does-not-exist", "screpdb")
		if inst := detectInstall(self); inst.supported || inst.reason != ReasonNotWritable {
			t.Errorf("got %+v, want unsupported not-writable", inst)
		}
	})

	t.Run("supported", func(t *testing.T) {
		buildinfo.Version = "v1.2.3"
		self := filepath.Join(t.TempDir(), "screpdb")
		inst := detectInstall(self)
		if !inst.supported || inst.reason != ReasonNone {
			t.Errorf("got %+v, want supported", inst)
		}
	})
}

func TestDirWritable(t *testing.T) {
	if !dirWritable(t.TempDir()) {
		t.Error("expected a fresh temp dir to be writable")
	}
	if dirWritable(filepath.Join(t.TempDir(), "missing")) {
		t.Error("expected a non-existent dir to be not writable")
	}
}

func TestCurrentAssetName(t *testing.T) {
	origVariant := buildinfo.Variant
	defer func() { buildinfo.Variant = origVariant }()

	buildinfo.Variant = "cli"
	got, ok := currentAssetName()
	want, wantOK := assetName(runtime.GOOS, runtime.GOARCH, "cli")
	if got != want || ok != wantOK {
		t.Errorf("currentAssetName() = (%q,%v), want (%q,%v)", got, ok, want, wantOK)
	}
}

func TestIsRestart(t *testing.T) {
	t.Setenv(restartEnv, "1")
	if !IsRestart() {
		t.Error("expected IsRestart true when env is set to 1")
	}
	t.Setenv(restartEnv, "0")
	if IsRestart() {
		t.Error("expected IsRestart false when env is not 1")
	}
}

func TestRestartEnvKV(t *testing.T) {
	kv := RestartEnvKV()
	if kv != restartEnv+"=1" {
		t.Errorf("RestartEnvKV() = %q, want %q", kv, restartEnv+"=1")
	}
	if !strings.HasSuffix(kv, "=1") {
		t.Errorf("RestartEnvKV() = %q, want it to mark the restart", kv)
	}
}

func TestExecutablePath(t *testing.T) {
	p, err := executablePath()
	if err != nil {
		t.Fatalf("executablePath: %v", err)
	}
	if !filepath.IsAbs(p) {
		t.Errorf("expected absolute path, got %q", p)
	}
}

func TestCleanupOldBinaryNoPanic(t *testing.T) {
	// Best-effort cleanup; with no stale ".old" file next to the test binary it
	// must be a harmless no-op rather than an error or panic.
	CleanupOldBinary()
}

func TestChecksumForMalformedHex(t *testing.T) {
	sums := []byte("nothex  screpdb-linux-amd64\n")
	if _, err := checksumFor(sums, "screpdb-linux-amd64"); err == nil {
		t.Fatal("expected error for non-hex checksum digest")
	}
}
