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
	"bo_11_hatch": true,
	"bo_12_hatch": true,
	// N Hatch <tech> composition markers (issue #245) — dynamic base count at the
	// economy→army transition, layered on top of the supply opener. Fixtures:
	// nhatch_hydra = bo_3hatch_hydra_pvz_pingcojerry(3) / _2jd(3) / bo_4hatch_hydra_pvz_syc(4);
	// nhatch_muta  = bo_2hmuta_tvz_*(2) / bo_z_3hatchmuta_chillibeans / _llIIll(3);
	// nhatch_lurker = bo_3hlurker_tvz_lyx2008 / _puuuuma(3) / _honjr(4).
	"nhatch_hydra":  true,
	"nhatch_muta":   true,
	"nhatch_lurker": true,
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
	// Round 10 batch 4 — composition/behavior markers watched & confirmed.
	"wraiths":      true, // bo_wraiths_tvz_1235sdfdfhg / _iilliii
	"muta_hitnrun": true, // bo_mutaharass_zvt_iliil (+ the F26080FE wraiths game)

	// Round 11 (remove remaining betas) — watched & confirmed by the user.
	// Fixtures: pvz_dblstargate_corsair_speedlot (P1 = Double Stargate + First
	// Corsair 6:21 + Speedlot completes 8:24), pvz_corsair_no_speedlot_gameend
	// (P1 = First Corsair 5:14, Speedlot must NOT fire — research unfinished at
	// game end), tvz_muta_turret_timing (Muta/Turret), bo_10hatch_money_dmarov
	// (P5 = 10 Hatch), bo_9pool9hatch_money_vortex (P4 = 9 Pool 9 Hatch),
	// money_ten_plus_scouts_denver94 (P1 = 10+ Scouts).
	"bo_10_hatch":     true,
	"bo_9_pool_hatch": true,
	"double_stargate": true,
	"first_corsair":   true,
	"speedlot_timing": true,
	"mutalisk_timing": true,
	"turret_timing":   true,
	"ten_plus_scouts": true,

	// Round 13 (issue #269) — watched & confirmed. Fixtures bo_10pool_zvz_mentalgap
	// (mentalgap = 10 Pool) and bo_tankless_expand_tvt_bisu (Bisu_chongchong =
	// Tankless Mech, expand-first). The round also fixed the gas/extractor-trick
	// undercount (AlgorithmVersion 60): the same-player 3hatch_hydra_2jd /
	// _pingcojerry fixtures now read 10 Hatch (was 4/6).
	"bo_10_pool":           true,
	"bo_t_tankless_expand": true,
	"bo_t_tankless_noexpa": true,

	// Round 13b/13c — corpus rescan (all corpora) for the still-beta Terran
	// fac-count buckets + 6 Pool, watched & confirmed. Regular-map fixtures:
	// bo_mech_expa_3fac_python_sabbath, bo_mech_expa_4fac_fs_sabbath,
	// bo_tankless_expa_3fac_cb_sabbath, bo_6pool_zvt_chobo85 (1v1). The N-fact
	// mech/goliath buckets otherwise fire only on Big Game Hunters (their natural
	// habitat); money-map fixtures: bo_goliath_expa_2fac_bgh_reflectingod,
	// bo_mech_expa_5fac_bgh_zenkiller, bo_goliath_noexpa_bgh_emoplugged.
	"bo_6_pool":               true,
	"bo_t_mech_expa_3fac":     true,
	"bo_t_mech_expa_4fac":     true,
	"bo_t_mech_expa_5fac":     true,
	"bo_t_tankless_expa_3fac": true,
	"bo_t_goliath_expa_2fac":  true,
	"bo_t_goliath_noexpa":     true,

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
	// Catch-all residual buckets, not detections: they claim whatever the named
	// rungs/openers leave over, so there is no premise to verify against a replay
	// and the "beta" tag only adds noise to an intentionally-unclassified label
	// ("Pool/Hatch (Other)", "Opener unresolved").
	"bo_zerg_other":     true,
	"bo_protoss_other":  true,
	"bo_terran_other":   true,
	"opener_unresolved": true,
}

// IsBetaExempt reports whether a marker is exempt from the beta tag because it
// is a deterministic measurement or a catch-all residual bucket, rather than a
// fallible detection with a premise to verify.
func IsBetaExempt(featureKey string) bool {
	return betaExemptFeatureKeys[featureKey]
}
