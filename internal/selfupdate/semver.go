package selfupdate

import (
	"strconv"
	"strings"
)

// Tier maps update urgency onto the version axis (issue #212): a newer major
// signals a curated "everyone should update" milestone (loud), while a newer
// minor/patch is an incremental nudge (quiet). It is decoupled from semver's
// usual "major = breaking" contract because screpdb has no library consumers.
type Tier string

const (
	TierNone  Tier = "none"
	TierQuiet Tier = "quiet"
	TierLoud  Tier = "loud"
)

type semver struct {
	major, minor, patch int
	ok                  bool
}

// parseSemver parses a vMAJOR.MINOR.PATCH string, tolerating a leading "v" and
// ignoring any pre-release/build metadata. Non-semver inputs (e.g. "dev" or a
// bare commit SHA) return ok=false.
func parseSemver(s string) semver {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return semver{}
	}
	nums := [3]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return semver{}
		}
		nums[i] = n
	}
	return semver{major: nums[0], minor: nums[1], patch: nums[2], ok: true}
}

func (a semver) cmp(b semver) int {
	switch {
	case a.major != b.major:
		return sign(a.major - b.major)
	case a.minor != b.minor:
		return sign(a.minor - b.minor)
	case a.patch != b.patch:
		return sign(a.patch - b.patch)
	default:
		return 0
	}
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	default:
		return 0
	}
}

// classifyTier compares the current build against the latest release and returns
// how loudly to nudge: loud for a newer major, quiet for a newer minor/patch,
// none when already current or when either version is unparseable.
func classifyTier(current, latest string) Tier {
	c := parseSemver(current)
	l := parseSemver(latest)
	if !c.ok || !l.ok {
		return TierNone
	}
	if l.cmp(c) <= 0 {
		return TierNone
	}
	if l.major > c.major {
		return TierLoud
	}
	return TierQuiet
}
