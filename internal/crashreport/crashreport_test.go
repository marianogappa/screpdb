package crashreport

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestWriteCreatesReportInWorkingDir(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	r := Capture("boom", []byte("stack here"))
	path, err := r.write()
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if base := filepath.Base(path); !strings.HasPrefix(base, "screpdb-crash-") || !strings.HasSuffix(base, ".log") {
		t.Errorf("unexpected report filename %q", base)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(data), "stack here") {
		t.Errorf("report file missing stack; got:\n%s", data)
	}
}

func TestFirstLine(t *testing.T) {
	cases := map[string]string{
		"":                     "unexpected panic",
		"  ":                   "unexpected panic",
		"single":               "single",
		"first\nsecond":        "first",
		strings.Repeat("x", 200): strings.Repeat("x", 120),
	}
	for in, want := range cases {
		if got := firstLine(in); got != want {
			t.Errorf("firstLine(%q) = %q, want %q", in, got, want)
		}
	}
}
