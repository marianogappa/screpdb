package crashreport

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/appdata"
)

func setTestAppData(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SCREPDB_APPDATA_DIR", dir)
	if _, err := appdata.Dir(); err != nil {
		t.Fatalf("appdata.Dir: %v", err)
	}
	return dir
}

func TestIssueURLPrefillsTitleAndBody(t *testing.T) {
	r := Capture("boom", []byte("goroutine 1 [running]:\nmain.main()"))
	raw := r.IssueURL()

	if !strings.HasPrefix(raw, newIssueURL+"?") {
		t.Fatalf("issue URL should target the new-issue endpoint, got %q", raw)
	}

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("issue URL is not parseable: %v", err)
	}
	q := u.Query()
	if got := q.Get("title"); got != "crash: boom" {
		t.Errorf("title = %q, want %q", got, "crash: boom")
	}
	if got := q.Get("labels"); got != "bug,crash" {
		t.Errorf("labels = %q, want %q", got, "bug,crash")
	}
	body := q.Get("body")
	for _, want := range []string{"boom", "goroutine 1 [running]:"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q; got:\n%s", want, body)
		}
	}
}

func TestIssueBodyTruncatesLongStack(t *testing.T) {
	longStack := strings.Repeat("a", maxStackChars+500)
	r := Capture("kaboom", []byte(longStack))
	body := r.issueBody()

	if strings.Contains(body, strings.Repeat("a", maxStackChars+1)) {
		t.Error("body should not contain the full over-length stack")
	}
	if !strings.Contains(body, "truncated") {
		t.Error("body should note that the stack was truncated")
	}
}

func TestFileTextContainsFullStack(t *testing.T) {
	longStack := strings.Repeat("z", maxStackChars+500)
	r := Capture("kaboom", []byte(longStack))
	if !strings.Contains(r.FileText(), longStack) {
		t.Error("crash report file should contain the full, untruncated stack")
	}
}

func TestWriteCreatesReportInCrashSubdir(t *testing.T) {
	dir := setTestAppData(t)

	r := Capture("boom", []byte("stack here"))
	path, err := r.write()
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if base := filepath.Base(path); !strings.HasPrefix(base, "screpdb-crash-") || !strings.HasSuffix(base, ".log") {
		t.Errorf("unexpected report filename %q", base)
	}
	crashDir := filepath.Join(dir, "crash")
	if filepath.Dir(path) != crashDir {
		t.Errorf("report written to %q, want inside crash subdir %q", filepath.Dir(path), crashDir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(data), "stack here") {
		t.Errorf("report file missing stack; got:\n%s", data)
	}
}

func TestGuardWithoutPanicReturns(t *testing.T) {
	Guard()
}

func TestSetOpenBrowserUpdatesDefault(t *testing.T) {
	prev := openBrowserDefault.Load()
	t.Cleanup(func() { openBrowserDefault.Store(prev) })

	SetOpenBrowser(true)
	if !openBrowserDefault.Load() {
		t.Error("SetOpenBrowser(true) did not set the default")
	}
	SetOpenBrowser(false)
	if openBrowserDefault.Load() {
		t.Error("SetOpenBrowser(false) did not clear the default")
	}
}

func TestGuardNonFatalRecoversAndRunsCleanup(t *testing.T) {
	setTestAppData(t)

	cleaned := false
	func() {
		defer GuardNonFatal(func() { cleaned = true })
		panic("boom")
	}()

	if !cleaned {
		t.Error("GuardNonFatal did not run the cleanup callback")
	}
}

func TestRetentionKeepsNewest(t *testing.T) {
	dir := setTestAppData(t)

	crashDir := filepath.Join(dir, "crash")
	if err := os.MkdirAll(crashDir, 0o755); err != nil {
		t.Fatal(err)
	}

	total := maxCrashFiles + 5
	for i := range total {
		f := filepath.Join(crashDir, fmt.Sprintf("screpdb-crash-20260101-%06d.log", i))
		if err := os.WriteFile(f, []byte("crash"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pruneOldReports(crashDir)

	entries, err := os.ReadDir(crashDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != maxCrashFiles {
		t.Errorf("after pruning: %d files, want %d", len(entries), maxCrashFiles)
	}
}

func TestFirstLine(t *testing.T) {
	cases := map[string]string{
		"":                       "unexpected panic",
		"  ":                     "unexpected panic",
		"single":                 "single",
		"first\nsecond":          "first",
		strings.Repeat("x", 200): strings.Repeat("x", 120),
	}
	for in, want := range cases {
		if got := firstLine(in); got != want {
			t.Errorf("firstLine(%q) = %q, want %q", in, got, want)
		}
	}
}
