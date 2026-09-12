package dashboard

import (
	"github.com/marianogappa/scfingerprint"
)

const (
	badgeKindFingerprint = "fingerprint"
	badgeKindAccount     = "account"
)

func (d *Dashboard) resolvePlayerIdentityBadges(playerKey string, fpMatch *workflowFingerprintMatch) (*IdentityBadge, *IdentityBadge) {
	fpBadge := fingerprintBadge(fpMatch)
	acctBadge := d.accountBadge(playerKey)
	return precedence(fpBadge, acctBadge)
}

// primaryIdentityBadge is the one badge a list row has room for: the winner of
// the precedence contest, with the losing claim dropped. The fingerprint match
// it needs is memoised per player key, so calling this once per row on a page
// costs one registry lookup per distinct player.
func (d *Dashboard) primaryIdentityBadge(playerKey string) *IdentityBadge {
	match, _ := d.matchFingerprint(playerKey, scfingerprint.FeatureVersion())
	primary, _ := d.resolvePlayerIdentityBadges(playerKey, match)
	return primary
}

func fingerprintBadge(m *workflowFingerprintMatch) *IdentityBadge {
	if m == nil || m.Tier == "" {
		return nil
	}
	return &IdentityBadge{
		Kind:         badgeKindFingerprint,
		Tier:         m.Tier,
		Label:        m.Label,
		Liquipedia:   m.Liquipedia,
		EvidenceKey:  "identity.badge.fingerprintEvidence",
		EvidenceArgs: []string{m.Label},
	}
}

func (d *Dashboard) accountBadge(playerKey string) *IdentityBadge {
	reg := d.fingerprintRegistry()
	if reg == nil {
		return nil
	}

	found, err := d.dbStore.BnetFoundProfilesForToon(d.ctx, playerKey)
	if err != nil || len(found) == 0 {
		return d.accountBadgeWithoutConfirmation(playerKey, reg)
	}

	for _, p := range found {
		acct, ok := reg.LookupToonOnGateway(playerKey, int(p.Gateway))
		if !ok {
			continue
		}
		return &IdentityBadge{
			Kind:         badgeKindAccount,
			Tier:         fingerprintTierConfirmed,
			Label:        acct.Name,
			Liquipedia:   "",
			EvidenceKey:  "identity.badge.accountConfirmedEvidence",
			EvidenceArgs: []string{acct.Name, scfingerprint.GatewayName(int(p.Gateway))},
		}
	}

	return d.accountBadgeWithoutConfirmation(playerKey, reg)
}

func (d *Dashboard) accountBadgeWithoutConfirmation(playerKey string, reg *scfingerprint.Registry) *IdentityBadge {
	acct, ok := reg.LookupToon(playerKey)
	if !ok {
		return nil
	}
	return &IdentityBadge{
		Kind:         badgeKindAccount,
		Tier:         fingerprintTierHigh,
		Label:        acct.Name,
		EvidenceKey:  "identity.badge.accountHighEvidence",
		EvidenceArgs: []string{acct.Name},
	}
}

var badgePrecedenceOrder = []struct {
	kind string
	tier string
}{
	{badgeKindFingerprint, fingerprintTierConfirmed},
	{badgeKindAccount, fingerprintTierConfirmed},
	{badgeKindFingerprint, fingerprintTierHigh},
	{badgeKindAccount, fingerprintTierHigh},
}

func badgeRank(b *IdentityBadge) int {
	if b == nil {
		return len(badgePrecedenceOrder)
	}
	for i, p := range badgePrecedenceOrder {
		if b.Kind == p.kind && b.Tier == p.tier {
			return i
		}
	}
	return len(badgePrecedenceOrder)
}

func precedence(fp, acct *IdentityBadge) (*IdentityBadge, *IdentityBadge) {
	if fp == nil && acct == nil {
		return nil, nil
	}
	if fp == nil {
		return acct, nil
	}
	if acct == nil {
		return fp, nil
	}
	if badgeRank(fp) <= badgeRank(acct) {
		return fp, acct
	}
	return acct, fp
}
