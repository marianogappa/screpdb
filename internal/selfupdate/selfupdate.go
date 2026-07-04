package selfupdate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/minio/selfupdate"

	"github.com/marianogappa/screpdb/internal/buildinfo"
)

// Status describes the result of a launch-time update check. It is serialized to
// the dashboard, which renders a loud or quiet notice based on Tier and offers a
// one-click update only when SelfUpdateSupported is true.
type Status struct {
	CurrentVersion      string `json:"current_version"`
	LatestVersion       string `json:"latest_version"`
	LatestReleaseURL    string `json:"latest_release_url"`
	Tier                Tier   `json:"tier"`
	UpdateAvailable     bool   `json:"update_available"`
	SelfUpdateSupported bool   `json:"self_update_supported"`
	Reason              string `json:"reason"`
	PackageManager      string `json:"package_manager"`
	OS                  string `json:"os"`
}

// CheckStatus performs the read-only launch-time check: it fetches the latest
// release, classifies the update tier, and reports whether this install can
// update itself in place. It never downloads or modifies the binary.
func CheckStatus(ctx context.Context) (Status, error) {
	s := Status{CurrentVersion: buildinfo.Version, Tier: TierNone, OS: runtime.GOOS}

	rel, err := fetchLatestRelease(ctx)
	if err != nil {
		return s, err
	}
	s.LatestVersion = rel.TagName
	s.LatestReleaseURL = rel.HTMLURL
	s.Tier = classifyTier(s.CurrentVersion, s.LatestVersion)
	s.UpdateAvailable = s.Tier != TierNone

	self, err := executablePath()
	if err != nil {
		s.Reason = ReasonNotWritable
		return s, nil
	}
	inst := detectInstall(self)
	s.Reason = inst.reason
	s.PackageManager = inst.manager
	if inst.supported {
		if name, ok := currentAssetName(); !ok {
			inst.supported = false
			s.Reason = ReasonUnsupported
		} else if _, ok := rel.asset(name); !ok {
			inst.supported = false
			s.Reason = ReasonUnsupported
		}
	}
	s.SelfUpdateSupported = inst.supported && s.UpdateAvailable
	return s, nil
}

// Apply downloads the latest release asset for this platform, verifies it
// against the minisign-signed SHA256SUMS, and atomically swaps the running
// binary. It returns the new version on success. The caller is expected to call
// Restart afterwards. Apply refuses to run on unsupported installs.
func Apply(ctx context.Context) (string, error) {
	self, err := executablePath()
	if err != nil {
		return "", err
	}
	inst := detectInstall(self)
	if !inst.supported {
		return "", fmt.Errorf("self-update not supported for this install (%s)", inst.reason)
	}
	name, ok := currentAssetName()
	if !ok {
		return "", fmt.Errorf("self-update not supported on this platform")
	}

	rel, err := fetchLatestRelease(ctx)
	if err != nil {
		return "", err
	}
	if classifyTier(buildinfo.Version, rel.TagName) == TierNone {
		return "", fmt.Errorf("already up to date (%s)", buildinfo.Version)
	}

	binAsset, ok := rel.asset(name)
	if !ok {
		return "", fmt.Errorf("release %s has no asset %s", rel.TagName, name)
	}
	sumsAsset, ok := rel.asset(checksumsAsset)
	if !ok {
		return "", fmt.Errorf("release %s has no %s", rel.TagName, checksumsAsset)
	}
	sigAsset, ok := rel.asset(signatureAsset)
	if !ok {
		return "", fmt.Errorf("release %s has no %s", rel.TagName, signatureAsset)
	}

	sumsData, err := downloadAsset(ctx, sumsAsset)
	if err != nil {
		return "", err
	}
	sigData, err := downloadAsset(ctx, sigAsset)
	if err != nil {
		return "", err
	}
	if err := verifySignature(sumsData, sigData); err != nil {
		return "", err
	}
	wantSum, err := checksumFor(sumsData, name)
	if err != nil {
		return "", err
	}

	binData, err := downloadAsset(ctx, binAsset)
	if err != nil {
		return "", err
	}

	// selfupdate.Apply recomputes the SHA-256 of binData and refuses the swap
	// unless it matches wantSum, so a tampered or truncated download is rejected
	// before the running binary is touched.
	if err := selfupdate.Apply(bytes.NewReader(binData), selfupdate.Options{
		TargetPath: self,
		Checksum:   wantSum,
	}); err != nil {
		if rollbackErr := selfupdate.RollbackError(err); rollbackErr != nil {
			return "", fmt.Errorf("update failed and rollback also failed (%v); the binary may need manual reinstall: %w", rollbackErr, err)
		}
		return "", fmt.Errorf("apply update: %w", err)
	}
	return rel.TagName, nil
}

// CleanupOldBinary removes the placeholder left next to the executable by a
// previous successful swap. On Unix the old inode is unlinked immediately; on
// Windows it lingers (the process was still running) until a later launch, which
// is when this runs. Best-effort: failures are ignored.
func CleanupOldBinary() {
	self, err := executablePath()
	if err != nil {
		return
	}
	dir := filepath.Dir(self)
	base := filepath.Base(self)
	_ = os.Remove(filepath.Join(dir, "."+base+".old"))
}

// executablePath returns the absolute, symlink-resolved path of the running
// binary — the file that will be swapped.
func executablePath() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		return resolved, nil
	}
	return self, nil
}
