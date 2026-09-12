package dashboard

import "testing"

func TestBadgePrecedenceOrder(t *testing.T) {
	for i := 0; i < len(badgePrecedenceOrder)-1; i++ {
		a := &IdentityBadge{Kind: badgePrecedenceOrder[i].kind, Tier: badgePrecedenceOrder[i].tier}
		b := &IdentityBadge{Kind: badgePrecedenceOrder[i+1].kind, Tier: badgePrecedenceOrder[i+1].tier}
		if badgeRank(a) >= badgeRank(b) {
			t.Errorf("%s-%s must outrank %s-%s", a.Kind, a.Tier, b.Kind, b.Tier)
		}
	}
}

func TestPrecedenceFingerprintConfirmedBeatsAccountConfirmed(t *testing.T) {
	fp := &IdentityBadge{Kind: badgeKindFingerprint, Tier: fingerprintTierConfirmed, Label: "Flash"}
	acct := &IdentityBadge{Kind: badgeKindAccount, Tier: fingerprintTierConfirmed, Label: "Flash"}
	primary, secondary := precedence(fp, acct)
	if primary.Kind != badgeKindFingerprint {
		t.Fatalf("primary should be fingerprint, got %s", primary.Kind)
	}
	if secondary.Kind != badgeKindAccount {
		t.Fatalf("secondary should be account, got %s", secondary.Kind)
	}
}

func TestPrecedenceAccountConfirmedBeatsFingerprintHigh(t *testing.T) {
	fp := &IdentityBadge{Kind: badgeKindFingerprint, Tier: fingerprintTierHigh, Label: "Flash"}
	acct := &IdentityBadge{Kind: badgeKindAccount, Tier: fingerprintTierConfirmed, Label: "Flash"}
	primary, _ := precedence(fp, acct)
	if primary.Kind != badgeKindAccount || primary.Tier != fingerprintTierConfirmed {
		t.Fatalf("confirmed account should beat high fingerprint, got %s-%s", primary.Kind, primary.Tier)
	}
}

func TestPrecedenceFingerprintHighBeatsAccountHigh(t *testing.T) {
	fp := &IdentityBadge{Kind: badgeKindFingerprint, Tier: fingerprintTierHigh, Label: "Flash"}
	acct := &IdentityBadge{Kind: badgeKindAccount, Tier: fingerprintTierHigh, Label: "Flash"}
	primary, _ := precedence(fp, acct)
	if primary.Kind != badgeKindFingerprint {
		t.Fatalf("fingerprint high should beat account high, got %s", primary.Kind)
	}
}

func TestPrecedenceNilsYieldNil(t *testing.T) {
	p, s := precedence(nil, nil)
	if p != nil || s != nil {
		t.Fatalf("both nil should yield nil, got %v %v", p, s)
	}
}

func TestPrecedenceSingleBadge(t *testing.T) {
	fp := &IdentityBadge{Kind: badgeKindFingerprint, Tier: fingerprintTierLead, Label: "X"}
	primary, secondary := precedence(fp, nil)
	if primary != fp || secondary != nil {
		t.Fatalf("single badge should be primary with nil secondary")
	}
	primary, secondary = precedence(nil, fp)
	if primary != fp || secondary != nil {
		t.Fatalf("single badge should be primary with nil secondary")
	}
}

func TestFingerprintBadgeNilOnNoMatch(t *testing.T) {
	if b := fingerprintBadge(nil); b != nil {
		t.Fatalf("expected nil, got %+v", b)
	}
	if b := fingerprintBadge(&workflowFingerprintMatch{Tier: ""}); b != nil {
		t.Fatalf("expected nil on empty tier, got %+v", b)
	}
}

func TestFingerprintBadgeSetsFields(t *testing.T) {
	m := &workflowFingerprintMatch{
		Label:      "Flash",
		Liquipedia: "https://liquipedia.net/starcraft/Flash",
		Tier:       fingerprintTierConfirmed,
	}
	b := fingerprintBadge(m)
	if b == nil {
		t.Fatal("expected badge")
	}
	if b.Kind != badgeKindFingerprint || b.Tier != fingerprintTierConfirmed || b.Label != "Flash" {
		t.Fatalf("unexpected badge: %+v", b)
	}
}
