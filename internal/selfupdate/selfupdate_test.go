package selfupdate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"aead.dev/minisign"
)

func TestParseSemver(t *testing.T) {
	cases := []struct {
		in   string
		want semver
	}{
		{"v1.2.3", semver{1, 2, 3, true}},
		{"1.2.3", semver{1, 2, 3, true}},
		{"v10.0.1-rc1", semver{10, 0, 1, true}},
		{"v1.2.3+build5", semver{1, 2, 3, true}},
		{"dev", semver{}},
		{"v1.2", semver{}},
		{"abc1234", semver{}},
		{"v1.2.x", semver{}},
	}
	for _, c := range cases {
		got := parseSemver(c.in)
		if got != c.want {
			t.Errorf("parseSemver(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestClassifyTier(t *testing.T) {
	cases := []struct {
		current, latest string
		want            Tier
	}{
		{"v1.2.3", "v1.2.3", TierNone},
		{"v1.2.3", "v1.2.4", TierQuiet},
		{"v1.2.3", "v1.3.0", TierQuiet},
		{"v1.2.3", "v2.0.0", TierLoud},
		{"v1.9.9", "v2.0.0", TierLoud},
		{"v2.0.0", "v1.9.9", TierNone}, // never downgrade
		{"dev", "v1.0.0", TierNone},    // dev build: no tier
		{"v1.0.0", "garbage", TierNone},
	}
	for _, c := range cases {
		if got := classifyTier(c.current, c.latest); got != c.want {
			t.Errorf("classifyTier(%q, %q) = %q, want %q", c.current, c.latest, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch, variant string
		want                  string
		ok                    bool
	}{
		{"windows", "amd64", "cli", "screpdb-windows-amd64.exe", true},
		{"windows", "amd64", "dashboard", "screpdb-dashboard-windows-amd64.exe", true},
		{"windows", "arm64", "cli", "", false},
		{"linux", "amd64", "cli", "screpdb-linux-amd64", true},
		{"linux", "arm64", "cli", "screpdb-linux-arm64", true},
		{"darwin", "amd64", "cli", "screpdb-darwin-amd64", true},
		{"darwin", "arm64", "cli", "screpdb-darwin-arm64", true},
		{"plan9", "amd64", "cli", "", false},
	}
	for _, c := range cases {
		got, ok := assetName(c.goos, c.goarch, c.variant)
		if got != c.want || ok != c.ok {
			t.Errorf("assetName(%q,%q,%q) = (%q,%v), want (%q,%v)", c.goos, c.goarch, c.variant, got, ok, c.want, c.ok)
		}
	}
}

func TestPackageManager(t *testing.T) {
	cases := []struct {
		self string
		want string
	}{
		{`C:\Users\me\scoop\apps\screpdb\current\screpdb.exe`, "scoop"},
		{"/home/me/scoop/apps/screpdb/current/screpdb", "scoop"},
		{"/opt/homebrew/Cellar/screpdb/1.2.3/bin/screpdb", "homebrew"},
		{"/home/linuxbrew/.linuxbrew/bin/screpdb", "homebrew"},
		{"/home/me/Downloads/screpdb", ""},
		{`C:\Tools\screpdb\screpdb.exe`, ""},
	}
	for _, c := range cases {
		if got := packageManager(c.self); got != c.want {
			t.Errorf("packageManager(%q) = %q, want %q", c.self, got, c.want)
		}
	}
}

func TestChecksumFor(t *testing.T) {
	sums := []byte("" +
		"aaaa  screpdb-linux-amd64\n" +
		"bbbb *screpdb-darwin-arm64\n" +
		"\n" +
		"deadbeef  SHA256SUMS\n")

	got, err := checksumFor(sums, "screpdb-linux-amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hex.EncodeToString(got) != "aaaa" {
		t.Errorf("got %x, want aaaa", got)
	}

	// Binary-mode "*" prefix is tolerated.
	if got, err := checksumFor(sums, "screpdb-darwin-arm64"); err != nil || hex.EncodeToString(got) != "bbbb" {
		t.Errorf("binary-mode entry: got %x err %v", got, err)
	}

	if _, err := checksumFor(sums, "screpdb-windows-amd64.exe"); err == nil {
		t.Error("expected error for missing asset")
	}
}

// TestVerifySignatureRejectsForgery confirms the verifier rejects content not
// signed by the embedded public key. A full positive test would require the
// release secret key, so we assert the negative path against the real embedded
// key plus a positive path against a locally generated key.
func TestVerifySignatureRejectsForgery(t *testing.T) {
	if err := verifySignature([]byte("not the real checksums"), []byte("untrusted comment: x\nRWQ\n")); err == nil {
		t.Error("expected verification to fail for unsigned/garbage data")
	}
}

func TestMinisignRoundTrip(t *testing.T) {
	// Validates the minisign Verify wiring with a freshly generated key, mirroring
	// how releases sign SHA256SUMS, independent of the embedded production key.
	pub, priv, err := minisign.GenerateKey(bytes.NewReader(bytes.Repeat([]byte{0x42}, 64)))
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	msg := []byte("hash  screpdb-linux-amd64\n")
	sig := minisign.Sign(priv, msg)
	if !minisign.Verify(pub, msg, sig) {
		t.Fatal("expected valid signature to verify")
	}
	if minisign.Verify(pub, append(msg, '!'), sig) {
		t.Fatal("expected tampered message to fail")
	}
}

func TestChecksumMatchesSHA256(t *testing.T) {
	// Guards the contract that selfupdate.Apply relies on: the digest parsed from
	// SHA256SUMS is a plain SHA-256 over the asset bytes.
	asset := []byte("fake binary contents")
	sum := sha256.Sum256(asset)
	line := hex.EncodeToString(sum[:]) + "  screpdb-linux-amd64\n"
	got, err := checksumFor([]byte(line), "screpdb-linux-amd64")
	if err != nil {
		t.Fatalf("checksumFor: %v", err)
	}
	if !bytes.Equal(got, sum[:]) {
		t.Errorf("parsed checksum does not match sha256(asset)")
	}
}
