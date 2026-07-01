package markers

// curatedFeatureKeys is the machine-readable mirror of the tier-1 ("human
// verified by watching the replay") premises documented in
// internal/patterns/GOLDEN_TIERS.md. A marker / build-order whose FeatureKey is
// NOT in this set has only auto-generated (tier-2) golden coverage — i.e. no
// human has eyeballed the detection against a real replay — and the dashboard
// flags it as "beta".
//
// Keep this in sync with GOLDEN_TIERS.md: when a detection is human-curated
// (a fixture + a documented premise), add its FeatureKey here so the beta tag
// disappears. Default is uncurated (beta) — the safe, honest default for any new
// detection.
var curatedFeatureKeys = map[string]bool{
	// Build-order openers — GOLDEN_TIERS.md "Build-order classification".
	// Zerg batch.
	"bo_11_hatch":        true,
	"bo_12_hatch":        true,
	"bo_z_2hatch_muta":   true,
	"bo_z_3hatch_lurker": true,
	"bo_z_2hatch_hydra":  true,
	// Round 8 (Zerg pool/hatch openers, issues #222/#223/#224) — fixtures
	// bo_z_*pool_*/bo_z_*hatch_* watched & confirmed.
	"bo_9_pool":     true,
	"bo_9_overpool": true,
	"bo_12_pool":    true,
	"bo_4_pool":     true,
	"bo_5_pool":     true,
	"bo_11_pool":    true,
	"bo_9_hatch":    true,
	"bo_13_hatch":   true, // fixture bo_z_13hatch_llllII (clean, unambiguous)

	// Round 8 — 3 Hatch Muta composition marker (converted from a BO opener);
	// fixtures bo_z_3hatchmuta_chillibeans (12 Hatch) / _llIIll (11 Hatch).
	"three_hatch_muta": true,

	// Round 8 — fuzzy Zerg opener (supply rung indeterminate from a multi-larva
	// morph). Fixtures bo_z_fuzzy_lllji (~11 Hatch), bo_z_fuzzy_foreigner70
	// (~12 Hatch), bo_z_fuzzy_overpool_bbbuuu (~9 Overpool).
	"bo_z_fuzzy": true,
	// Round 3 (Protoss/Terran).
	"bo_cc_first":       true,
	"bo_t_bio_1base":    true,
	"bo_t_bio_2base":    true,
	"bo_p_1gate_reaver": true,
	// Round 4 (Protoss).
	"bo_1_gate_core": true,
	"bo_2_gate":      true,
	"bo_nexus_first": true,
	"bo_gate_expand": true,
	"bo_forge_expa":  true,
	// Round 5 (Protoss cannon-contain) — only the two permutations with fixtures.
	"bo_p_gate_forge_cannon": true,
	"bo_p_forge_cannon_gate": true,
	// Round 6 (Terran air/specialist). "2 Port Wraith" → "2 Starport Wraith"
	// (round-9 rename); the standalone "Goliath" folded into the mech
	// composition flavor — its fixtures (iilliii1/lilliill = 1 Fact, f1ssasad =
	// 3 Fact before expa) now classify under the Goliath flavor keys.
	"bo_bbs":                 true,
	"bo_t_2starport_wraith":  true,
	"bo_t_goliath_expa_1fac": true,
	"bo_t_goliath_expa_3fac": true,
	// bo_team_mech_111.rep tier-1 per-player premises (round 9 re-curation:
	// chobo86's "5-Fac Mech" → "1 Fact Expa Mech" — expanded after 1 Factory,
	// then ramped; ALT+F4's "1-1-1 into Mech" → "1-1-1 Mech"). The retired
	// 2 Port Wraith / 2 Fact before Expa premises now map to "2 Starport Wraith"
	// / "2 Fact Expa Mech" and are re-curated in the round-9 watch pass.
	"bo_t_mech_expa_1fac": true,
	"bo_t_111_mech":       true,

	// Round 9 (Terran mech taxonomy, issues #226/#227) — 27 ladder replays
	// watched & confirmed, one fixture per name (bo_<bo>_<mu>_<player>.rep).
	"bo_t_mech_expand":      true, // "Mech" (expand-first)
	"bo_t_goliath_expand":   true, // "Goliath" (expand-first)
	"bo_t_2starport_valk":   true,
	"bo_t_3starport_wraith": true,
	"bo_bunker_rush":        true,
	"bo_t_111":              true,
	"bo_t_111_tankless":     true,

	// Round 10 (remove-betas pass) — watched & confirmed ladder replays.
	"bo_1_gate_no_expa":   true, // bo_1gatenoexpa_pvz_566 / _pvt_broodwarisbest
	"bo_7_pool":           true, // bo_7pool_zvt_3050sdsd / _zvp_herwater
	"bo_8_pool":           true, // bo_8pool_zvt_coffeegene / _zvt_loveaddio
	"bo_t_3starport_valk": true, // bo_3starport_valk_tvz_as2qs
	// Round 10 batch 2.
	"carriers":                true, // bo_carriers_pvt_vncgsncs
	"battlecruisers":          true, // bo_bcs_tvz_1246768854333 / _lIIIlIllIlIll
	"bo_forge_cannon_no_expa": true, // bo_forgecannon_noexpa_pvz_lyx2008 / _liiliil
	"bo_p_forge_gate_cannon":  true, // bo_forgegatecannon_pvz_lllilliii
	"bo_t_mech_expa_2fac":     true, // bo_mech_expa_2fac_tvp_f1ssasad
	"threw_nukes":             true, // bo_nukes_tvp_iliilii / _tvz_vvwv
	"sair_speedlot":           true, // bo_sairspeedlot_pvz_tomsonnet / _tufbeombu
	"bo_t_tankless_expa_1fac": true, // bo_tankless_expa_1fac_tvp_f1ssasad / _tvz_wicobaduk2
	"wraith_cloak_timing":     true, // bo_wraithcloak_tvz_1235sdfdfhg / _llllilllilll
	"bo_t_mech_noexpa":        true, // "1-Base Mech": bo_1base_mech_tvp_namu / _wjddsu

	// Round 7 — fixtures crazy_zerg_guardians_tvz_lyx2008, maelstrom_pvz_bysnow,
	// first_observer_pvt_0sawon, first_mine_pvt_f1ssasad.
	"crazy_zerg":     true,
	"guardians":      true,
	"made_maelstrom": true,
	"first_observer": true,
	"first_mine":     true,

	// Signature / event markers with a tier-1 fixture premise.
	"manner_pylon":    true, // GOLDEN_TIERS.md "Manner pylon"
	"first_reaver":    true, // guarded in the manner_pylon fixture
	"cliff_drop":      true, // GOLDEN_TIERS.md "Cliff-drop detection"
	"made_drops":      true, // GOLDEN_TIERS.md "Drops"
	"made_recalls":    true, // GOLDEN_TIERS.md "Recall target inference"
	"offensive_nydus": true, // GOLDEN_TIERS.md "Offensive-nydus detection"
}

// IsCurated reports whether the marker / build-order with this FeatureKey has a
// human-curated (tier-1) golden premise. Uncurated detections are surfaced as
// "beta" in the dashboard.
func IsCurated(featureKey string) bool {
	return curatedFeatureKeys[featureKey]
}

// betaExemptFeatureKeys are markers that should never carry the "beta" tag even
// though they have no tier-1 golden — they are exact, deterministic
// measurements (hotkey-group usage), not fallible pattern detections, so there
// is nothing for a human to verify against a replay.
var betaExemptFeatureKeys = map[string]bool{
	"used_hotkey_groups": true,
	"never_used_hotkeys": true,
	// Deterministic facts / phase boundaries, not fallible detections — there is
	// nothing to verify against a replay, so they carry no "beta" tag (round 10).
	"became_terran":         true,
	"became_zerg":           true,
	"late_game_starts":      true,
	"mid_game_starts":       true,
	"viewport_multitasking": true,
	"never_researched":      true,
	"never_upgraded":        true,
}

// IsBetaExempt reports whether a marker is exempt from the beta tag because it
// is a deterministic measurement rather than a verifiable detection.
func IsBetaExempt(featureKey string) bool {
	return betaExemptFeatureKeys[featureKey]
}
