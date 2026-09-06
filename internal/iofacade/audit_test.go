package iofacade_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// auditEntry matches a dated log line in the README's I/O Safety Audit block.
// The log is rendered as a fenced code block, so an entry is a line that starts
// with a date, e.g. "2026-05-31  OK. ...". The older bullet form
// ("- **2026-05-31** — ...") still matches for backward compatibility.
var auditEntry = regexp.MustCompile(`(?m)^(?:- \*\*)?\d{4}-\d{2}-\d{2}`)

// TestIOSafetyAuditPresent fails CI when the README's I/O Safety Audit log is
// empty (issue #135). The audit itself is a best-effort verdict written by the
// LLM agent that authors a change; this test only enforces that *some* verdict
// is present — a change cannot land with an empty log. It deliberately does not
// check freshness (a per-commit timestamp check would be flaky), so it is not a
// substitute for the authoritative TestNoDirectIOOutsideFacades guard.
func TestIOSafetyAuditPresent(t *testing.T) {
	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}

	const start = "<!-- IO-AUDIT:START -->"
	const end = "<!-- IO-AUDIT:END -->"
	text := string(data)
	i := strings.Index(text, start)
	j := strings.Index(text, end)
	if i < 0 || j < 0 || j < i {
		t.Fatalf("README.md is missing the I/O Safety Audit markers %q / %q", start, end)
	}

	block := text[i+len(start) : j]
	if !auditEntry.MatchString(block) {
		t.Fatalf("README.md I/O Safety Audit log has no dated entry between the markers; "+
			"the authoring LLM must add a line like \"YYYY-MM-DD  OK. <justification>\" (issue #135). Got:\n%s",
			strings.TrimSpace(block))
	}
}

// TestIOSafetyAuditSingleNonCollapsedEntry keeps the top-level audit log to a
// single (latest) entry: every change appends a fresh line and pushes the
// previous ones down into the collapsed "Older ... entries" <details>. Without
// this guard the visible log grows unbounded, which repeatedly happens by
// accident. TestIOSafetyAuditArchiveBounded caps the collapsed archive.
func TestIOSafetyAuditSingleNonCollapsedEntry(t *testing.T) {
	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}

	const start = "<!-- IO-AUDIT:START -->"
	const end = "<!-- IO-AUDIT:END -->"
	text := string(data)
	i := strings.Index(text, start)
	j := strings.Index(text, end)
	if i < 0 || j < 0 || j < i {
		t.Fatalf("README.md is missing the I/O Safety Audit markers %q / %q", start, end)
	}

	// The non-collapsed log is everything from the START marker up to the
	// collapsed "Older ... entries" <details>; the archive inside <details> is
	// exempt. If there is no <details>, the whole block is non-collapsed.
	block := text[i+len(start) : j]
	nonCollapsed := block
	if d := strings.Index(block, "<details>"); d >= 0 {
		nonCollapsed = block[:d]
	}
	entries := auditEntry.FindAllString(nonCollapsed, -1)
	if len(entries) > 1 {
		t.Fatalf("README.md I/O Safety Audit has %d non-collapsed dated entries, want exactly 1; "+
			"keep only the latest line above the <details> and move the older ones into the collapsed "+
			"\"Older I/O safety audit entries\" section. Got:\n%s",
			len(entries), strings.TrimSpace(nonCollapsed))
	}
}

// auditArchiveMax is how many entries the collapsed archive keeps. The audit is
// a receipt that makes tampering visible in the diff, and git history is the
// real record, so the README only needs enough recent context to read the
// current entry against. The log had accreted to 73 entries across 27 separate
// fenced blocks before this bound existed.
const auditArchiveMax = 5

// TestIOSafetyAuditArchiveBounded caps the collapsed "Older I/O safety audit
// entries" archive at auditArchiveMax. Adding an entry means promoting it to
// the visible slot and dropping the oldest archived one, so the log stays a
// fixed size instead of growing without limit.
func TestIOSafetyAuditArchiveBounded(t *testing.T) {
	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}

	const start = "<!-- IO-AUDIT:START -->"
	const end = "<!-- IO-AUDIT:END -->"
	text := string(data)
	i := strings.Index(text, start)
	j := strings.Index(text, end)
	if i < 0 || j < 0 || j < i {
		t.Fatalf("README.md is missing the I/O Safety Audit markers %q / %q", start, end)
	}

	block := text[i+len(start) : j]
	d := strings.Index(block, "<details>")
	if d < 0 {
		// No archive yet is fine: the visible entry is the whole log.
		return
	}
	archived := auditEntry.FindAllString(block[d:], -1)
	if len(archived) > auditArchiveMax {
		t.Fatalf("README.md I/O Safety Audit archive has %d entries, want at most %d; "+
			"when you add an entry, promote it to the visible slot above the <details> and drop "+
			"the oldest archived line (git history keeps the full record)",
			len(archived), auditArchiveMax)
	}
}
