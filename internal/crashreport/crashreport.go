// Package crashreport turns an otherwise-silent panic into something a
// non-technical tester can act on: it writes a crash report file next to the
// binary, prints a pre-filled GitHub "new issue" link, and (for GUI builds with
// no visible console) opens that link in the browser. See issue #165.
package crashreport

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/buildinfo"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/pkg/browser"
)

// newIssueURL is the GitHub endpoint that opens a blank issue editor. We
// pre-fill the title/body via query params rather than pointing at a specific
// issue-form template so the link keeps working regardless of template changes
// (blank issues must stay enabled in .github/ISSUE_TEMPLATE/config.yml).
const newIssueURL = "https://github.com/marianogappa/screpdb/issues/new"

// maxStackChars caps the stack trace embedded in the issue URL. The full,
// untruncated trace always lands in the crash report file; this only keeps the
// pre-filled URL comfortably under GitHub's length limit.
const maxStackChars = 4000

// Report captures everything known about a crash at the moment it happened.
type Report struct {
	When    time.Time
	Version string
	Commit  string
	GOOS    string
	GOARCH  string
	Cause   string // the recovered panic value, formatted
	Stack   string // full goroutine stack from debug.Stack()
}

// Recover is meant to be deferred at the top of main() or a long-lived
// goroutine:
//
//	defer crashreport.Recover(false)
//
// On a non-nil panic it writes a crash report, surfaces a pre-filled issue link
// (opening the browser when openBrowser is true, for GUI builds), and exits the
// process. On a clean return it does nothing.
func Recover(openBrowser bool) {
	recovered := recover()
	if recovered == nil {
		return
	}
	Handle(recovered, debug.Stack(), openBrowser)
	os.Exit(2)
}

// Handle processes an already-recovered panic. It is exported (separately from
// Recover) so callers that capture the stack themselves, and tests, can drive
// it directly. It never re-panics.
func Handle(recovered any, stack []byte, openBrowser bool) {
	r := Capture(recovered, stack)
	path, writeErr := r.write()
	issueURL := r.IssueURL()

	const rule = "──────────────────────────────────────────────────────────────"
	fmt.Fprintln(os.Stderr, "\n"+rule)
	fmt.Fprintln(os.Stderr, "screpdb crashed — sorry about that!")
	fmt.Fprintf(os.Stderr, "Build: %s (%s) on %s/%s\n", r.Version, r.Commit, r.GOOS, r.GOARCH)
	if writeErr == nil {
		fmt.Fprintf(os.Stderr, "A crash report was saved to:\n  %s\n", path)
	}
	fmt.Fprintln(os.Stderr, "\nPlease help fix this by opening a pre-filled issue:")
	fmt.Fprintf(os.Stderr, "  %s\n", issueURL)
	fmt.Fprintln(os.Stderr, rule)

	if openBrowser {
		_ = browser.OpenURL(issueURL)
	}
}

// Capture builds a Report from a recovered panic value and a stack trace.
func Capture(recovered any, stack []byte) Report {
	return Report{
		When:    time.Now().UTC(),
		Version: buildinfo.Version,
		Commit:  buildinfo.Commit,
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
		Cause:   fmt.Sprint(recovered),
		Stack:   string(stack),
	}
}

// FileText is the full, untruncated crash report written to disk.
func (r Report) FileText() string {
	return fmt.Sprintf(`screpdb crash report
====================
time:    %s
version: %s
commit:  %s
os/arch: %s/%s

panic: %s

%s
`, r.When.Format(time.RFC3339), r.Version, r.Commit, r.GOOS, r.GOARCH, r.Cause, r.Stack)
}

// IssueURL builds a GitHub "new issue" link with the title and body pre-filled
// from the crash details, so a tester only has to click and submit.
func (r Report) IssueURL() string {
	q := url.Values{}
	q.Set("title", "crash: "+firstLine(r.Cause))
	q.Set("labels", "bug,crash")
	q.Set("body", r.issueBody())
	return newIssueURL + "?" + q.Encode()
}

func (r Report) issueBody() string {
	stack := r.Stack
	if len(stack) > maxStackChars {
		stack = stack[:maxStackChars] + "\n…(truncated — full stack is in the crash report file)…"
	}
	var b strings.Builder
	b.WriteString("**What happened?**\n")
	b.WriteString("screpdb crashed. Please describe what you were doing when it happened, and attach the replay if one was involved.\n\n")
	fmt.Fprintf(&b, "**Version:** %s (%s)\n", r.Version, r.Commit)
	fmt.Fprintf(&b, "**OS / Arch:** %s/%s\n\n", r.GOOS, r.GOARCH)
	b.WriteString("**Panic**\n```\n")
	b.WriteString(r.Cause)
	b.WriteString("\n```\n\n**Stack trace**\n```\n")
	b.WriteString(stack)
	b.WriteString("\n```\n")
	return b.String()
}

// write saves the full report next to the binary (the working directory, an
// always-permitted iofacade root) and returns its absolute path.
func (r Report) write() (string, error) {
	name := "screpdb-crash-" + r.When.Format("20060102-150405") + ".log"
	if err := iofacade.WriteFile(name, []byte(r.FileText()), 0o644); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return name, nil
	}
	return abs, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 120 {
		s = s[:120]
	}
	if s == "" {
		return "unexpected panic"
	}
	return s
}
