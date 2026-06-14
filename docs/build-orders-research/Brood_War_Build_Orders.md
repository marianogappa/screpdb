# StarCraft: Brood War / Remastered — Build-Order Reference for Replay Detection

**Purpose:** reference data for improving build-order (BO) detection in `screpdb`. Every entry is a **single, disjoint opening** described as a **building timeline** — the signal a replay parser detects reliably. Worker/unit production is deliberately de-emphasised (it's noisy/duplicated in parsed replays). Brood War only — no StarCraft 2.

**Compiled:** June 2026. Sources: Liquipedia Brood War wiki (build-order pages, read with a JS renderer to capture the templated tables) + recent ASL pro games.

---

## ⚠️ The single most important finding: BW builds are keyed to SUPPLY, not seconds

You asked for "seconds from game start." After reading ~40 Liquipedia BO pages directly, here is the honest reality you need to design around:

**Brood War build orders are almost universally expressed in SUPPLY counts** (e.g. "Forge @11, Nexus @15"), occasionally in **resource thresholds** (`@100 Gas`, `@400 minerals`) or **build-progress** (`@100% Lair`, `@Spire 320 HP`). **Absolute game-clock timings (m:ss) are rare** — only a handful of pages publish any.

Per your selection ("only sourced timings"), I did **not** fabricate or estimate any seconds. So:

- **Building SEQUENCE + SUPPLY is fully populated** for almost every build (this is the strong, sourced fingerprint).
- **Sourced game-clock seconds exist for only ~7 builds** — consolidated in the table below.

### Why this is still the most useful thing for your matcher

Your parser already extracts **exact timestamps for each building**. What it lacks is the *reference* to match against. Two practical implications:

1. **The building sequence itself is the primary fingerprint.** Order + identity of structures (e.g. *Forge → Nexus → Gateway → Cannon → Cyber* vs *Gateway → Cyber → Citadel → Templar Archives*) discriminates builds without needing supply at all — and you have exact times for each. The supply numbers below tell you the *expected ordering and rough spacing*.
2. **Supply is itself noisy in your data** (it's derived from worker/unit counts, which you said are spammy). So a supply-keyed reference is only loosely matchable. **The robust path to second-level reference timings is to derive them empirically from your own labelled replay corpus** — i.e. take N pro replays known to be build X, and compute the median game-clock timestamp of each building. Your tool is uniquely positioned to do this; the building_events.csv companion gives you the schema to populate. (Happy to help script this against your repo.)

### Consolidated SOURCED game-clock timings (verbatim from Liquipedia)

These are the only absolute m:ss timings I could source. Use them as hard anchors.

| Build | Event | Sourced timing (verbatim) |
|---|---|---|
| T9 — 5 Factory Goliaths (vZ) | should not lose units until "55-65 supply mark" | **"around 6:30"** |
| T9 — 5 Factory Goliaths (vZ) | Vehicle Plating completes | **"~7:30"** (at 75 supply) |
| T9 — 5 Factory Goliaths (vZ) | final move-out | **"~25 seconds after starting Siege Mode"** |
| T13 — 2 Fact Vults (vT) | Refinery placement | **"before or right on the 1:45 mark... 2 seconds right after your 12th SCV pops"** |
| T7 — +1 Sair/Speedlot¹ (vZ) | Robotics Facility + Singularity Charge | **"at around 9:00"** |
| Z8 — 3 Hatch Muta (vT) | Spire placement | **"between 4:48-4:52, depending on the opening build"** |
| Z9 — 2 Hatch Muta (vT) | Mutalisks complete (unit, not building) | **"around 6:30"** (12 Hatch) / **"around 6:00"** (12 Pool) |
| Z15 — 12 Pool (vZ) | Lair start | **"around 3 minutes into the game"** |
| P12 — 2 Gate Reaver (PvP) | Observer / Range / Reaver+Shuttle / Nexus | **Observer ~5:30–6:40; Range ~5:40–7:00; Reaver+Shuttle ~7:30–8:00; push ~7:50–8:00; Nexus ~8:40** (varies by variant — see entry) |
| Z1 — 3 Base Spire→5 Hatch Hydra (vP) | army-supply checkpoint (not a building) | **"at the 9 minute mark you should have 85 supply"** |
| Z7 — 12 Hatch (vT) | 13-Pool variant pool delay (relative) | **"delays Spawning Pool by around 10-12 seconds"** |

¹ The +1 Sair/Speedlot entry lives under Protoss; the "9:00" tag is its only clock mark.

> **Notation in the tables below:** Supply is shown as `@N`. `@100% X` = on completion of X. `@N gas` / `@N min` = resource threshold. A **Clock** column is filled only where a source gives an absolute m:ss; otherwise `—`.

---
---

# TERRAN — 14 openings

## TvP

### T1 · Siege Expand (시즈 앞마당) — 🟢 High · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Supply Depot|9|—|
|2|Barracks|12|—|
|3|Refinery|12|—|
|4|Supply Depot|15|—|
|5|Factory|16|—|
|6|Machine Shop|@100% Factory|—|
|7|Command Center (nat)|21|—|
|8|Supply Depot|24|—|
|9|Engineering Bay (vs DT/drop, optional)|28|—|

Safest TvP opener; entry into the mech game. **Source:** [Siege Expand (vs. Protoss)](https://liquipedia.net/starcraft/Siege_Expand_(vs._Protoss)). Live in every modern pro TvP mech game (Flash 2025–26).

### T2 · Double Armory / Flash Build (더블 아머리) — 🟢 High · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Supply Depot|9|—|
|2|Barracks|11|—|
|3|Refinery|11|—|
|4|Factory|15|—|
|5|Supply Depot|15|—|
|6|Command Center (nat)|21|—|
|7|Supply Depot|24|—|
|8|Armory|33|—|
|9|Engineering Bay|33|—|
|10|Supply Depot|38 / 46 / 49|—|
|11|Factory #2|49|—|
|12|Command Center #3|50|—|
|13|Starport|@50% Vehicle Weapons L1|—|
|14|Science Facility|@100% Starport|—|
|15|Armory #2 + Academy|@100% Starport|—|
|16|Comsat Stations (both CCs)|@100% Academy|—|

The defining modern TvP macro-mech build (Flash). **Source:** [Double Armory (vs. Protoss)](https://liquipedia.net/starcraft/Double_Armory_(vs._Protoss)). Recent: [Flash vs Stork, Oct 2025](https://www.youtube.com/watch?v=Au9WIumyZEg); [Flash vs Bisu, 2026](https://www.youtube.com/watch?v=xWfITZe7NO4).

### T3 · 1 Rax FE (vs. Protoss) (원배럭 더블) — 🟡 Medium · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Supply Depot|9|—|
|2|Barracks|11|—|
|3|Command Center (nat)|15 (fast) / 17 (safe)|—|
|4|Refinery|16 / 18|—|
|5|Factory|25|—|
|6|Supply Depot|28|—|
|7|Refinery (nat)|30|—|
|8|Engineering Bay (safe var.)|25→ siege @28|—|

More economic FE; punishes Protoss greed. **Source:** [1 Rax FE (vs. Protoss)](https://liquipedia.net/starcraft/1_Rax_FE_(vs._Protoss)).

### T4 · FD / Fake Double (페이크 더블) — 🟡 Medium · Type: Opening
| # | Building | Supply / trigger | Clock |
|---|---|---|---|
|1|Supply Depot|9/10|—|
|2|Barracks|11 (= Refinery same time)|—|
|3|Refinery|11|—|
|4|Factory|15–16|—|
|5|Supply Depot|16|—|
|6|Machine Shop|@100% Factory|—|
|7|Command Center (nat)|@400 minerals|—|

Fakes 2-Fact pressure; secures FE while punishing fast tech. Variants (Hwasin, Strong FD/Flash) shift gas-cut timing. **Source:** [FD (vs. Protoss)](https://liquipedia.net/starcraft/FD_(vs._Protoss)). *No absolute clock (Hwasin leaves "10–20s sooner", relative).*

## TvZ

### T6 · 1 Rax FE (vs. Zerg) (원배럭 더블) — 🟢 High · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Supply Depot|9/10|—|
|2|Barracks (+scout)|11|—|
|3|Command Center (nat)|15–18 (15 vs Hatch-first; cancel vs 12 Hatch)|—|

The universal TvZ opener; branches into all builds below. **Source:** [1 Rax FE (vs. Zerg)](https://liquipedia.net/starcraft/1_Rax_FE_(vs._Zerg)). Used by every Terran in ASL 19/20/21.

### T7 · +1 Weapons "Up-Terran" (업테란) — 🟢 High · Type: Opening (off 1 Rax FE)
| # | Building | Supply / trigger | Clock |
|---|---|---|---|
|1|(1 Rax FE base)|—|—|
|2|Barracks #2|~22|—|
|3|Refinery|after Rax #2|—|
|4|Engineering Bay|after Refinery|—|
|5|+1 Weapons|@100% Ebay|—|
|6|Academy|after +1 starts|—|
|7|Barracks #3, #4|after Starport|—|

+1 lands as mutas arrive. Flash's signature TvZ bio. **Source:** [1 Rax FE (vs. Zerg) — Upgrade variant](https://liquipedia.net/starcraft/1_Rax_FE_(vs._Zerg)).

### T8 · +1 5 Rax (플러스원 5배럭) — 🟡 Medium · Type: Opening (anti-3-Hatch-Muta)
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Supply Depot|9|—|
|2|Barracks|11|—|
|3|Supply Depot|15|—|
|4|Command Center|18|—|
|5|Refinery → Supply Depot → Engineering Bay|—|—|
|6|+1 Weapons|@100% Ebay|—|
|7|Bunker → Academy|—|—|
|8|Barracks ×4 (5 total)|—|—|
|9|Comsats; move out (16–20 Marines + 4–5 Medics)|—|—|

From-1RaxFE variant: Depot@22, Refinery@23, Ebay@25, Academy@30, +1@100%Ebay, Rax@32, Rax@35, 2 Rax@40. **Source:** [+1 5 Rax](https://liquipedia.net/starcraft/%2B1_5_Rax). Do **not** use vs 2-Hatch.

### T9 · 5 Factory Goliaths (5팩 골리앗) — 🟢 High · Type: Opening · Creator: Flash · ⏱ has clock anchors
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Supply Depot|9/10|—|
|2|Barracks|11|—|
|3|Refinery|12|—|
|4|Supply Depot|16|—|
|5|Factory|17|—|
|6|Command Center (nat)|20|—|
|7|Vultures ×1–3|@100% Factory|—|
|8|Armory|24|—|
|9|Factory #2|29|—|
|10|Machine Shop (+1 Armor, then Goliaths nonstop)|@100% Machine Shop|**~6:30 @55–65 supply**|
|11|Factory #3|39|—|
|12|Engineering Bay|44|—|
|13|Missile Turrets (1–2/base)|55|—|
|14|Vehicle Plating completes|75|**~7:30**|
|15|Factory #4, #5|78|—|
|16|Academy → Siege Tanks (1–3)|82–86|—|
|17|2 Comsats; Siege Mode|102|—|
|18|Move out|104|**~25s after Siege Mode**|
|19|Command Center #3 (expand behind push)|111|—|

Mech that hard-counters muta. **Source:** [5 Factory Goliaths (vs. Zerg)](https://liquipedia.net/starcraft/Flash%27s_5_Factory_Goliath_Build(vs._Zerg)) + [Flash's Eng-subbed guide](https://www.youtube.com/watch?v=QZRpEev4t_8). Recent: [ASL S21 Flash vs Jaedong, Apr 2026](https://www.youtube.com/watch?v=CtMFmBDf1tI).

### T11 · Aggressive 4-Rax vs Economic Muta — 🟡 Medium · Type: Opening (off 1 Rax FE)
| # | Building | Order | Clock |
|---|---|---|---|
|1|(1 Rax FE base)|—|—|
|2|Barracks ×2 (4 total)|after turrets/upgrades|—|
|3|Factory|—|—|
|4|Starport (dropship)|—|—|
|5|Science Facility + Engineering Bay #2|—|—|

Punishes greedy no-ling muta. **Source:** [1 Rax FE (vs. Zerg) — 4 Rax variation](https://liquipedia.net/starcraft/1_Rax_FE_(vs._Zerg)).

## TvT

### T12 · 1 Fact FE (vs. Terran) (원팩 더블) — 🟢 High · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Supply Depot|9/10|—|
|2|Barracks|12|—|
|3|Refinery|12|—|
|4|Supply Depot|16|—|
|5|Factory|16|—|
|6|Command Center (nat)|20|—|
|7|Supply Depot|23|—|
|8|Machine Shop|24|—|
|9|Factory #2|26|—|

Standard TvT macro. **Source:** [1 Fact FE (vs. Terran)](https://liquipedia.net/starcraft/1_Fact_FE_(vs._Terran)).

### T13 · 2 Fact Vults (vs. Terran) (투팩 벌처) — 🟡 Medium · Type: Opening · ⏱ has clock anchor
| # | Building | Supply / trigger | Clock |
|---|---|---|---|
|1|Barracks|11–12 (@150 min)|—|
|2|Refinery|@12th SCV|**"before/at 1:45 mark, ~2s after 12th SCV pops"**|
|3|Factory|16 (Vulture Speed first)|—|
|4|Factory #2 + Machine Shop #2|—|—|
|5|(both Speed + Spider Mines)|—|—|
|6|Engineering Bay/Armory (vs wraith/drop)|28|—|

Aggressive vulture/mine timing vs FE/14CC. **Source:** [2 Fact Vults (vs. Terran)](https://liquipedia.net/starcraft/2_Fact_Vults_(vs._Terran)). *Full step table renders only the prose clarification; supply for later steps approximate.*

### T14 · 3 Fact Vulture (vs. Terran) (쓰리팩 벌처) — 🟡 Medium · Type: Opening · Creator: Themarine
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Supply Depot|9/10|—|
|2|Barracks|11|—|
|3|Refinery (scout)|11|—|
|4|Supply Depot|15|—|
|5|Factory|18|—|
|6|Factory #2|20|—|
|7|Vulture + Machine Shop|@100% Factory|—|
|8|Supply Depot|22|—|
|9|Vulture + Speed|25|—|
|10|Machine Shop #2|27|—|
|11|Supply Depot|30|—|
|12|Factory #3 + Spider Mines|32|—|

Deadlier (less flexible) vulture/mine timing; hard-counters 2-Port Wraith / 14 CC. **Source:** [3 Fact Vulture (vs. Terran)](https://liquipedia.net/starcraft/3_Fact_Vulture_(vs._Terran)).

### T15 · 2 Port Wraith (vs. Terran) (투포트 레이스) — 🟡 Medium · Type: Opening · Popularized: Flash
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Supply Depot|9/10|—|
|2|Barracks|11|—|
|3|Refinery|12|—|
|4|Supply Depot|13|—|
|5|Factory|16|—|
|6|Starport ×2|22|—|
|7|Supply Depot|22|—|
|8|Supply Depot|30|—|

Cloaked-wraith harass into mech. **Source:** [2 Port Wraith (vs. Terran)](https://liquipedia.net/starcraft/2_Port_Wraith_(vs._Terran)) (Flash beat Classic, IeSF 2009).

### T16 · 14 CC (vs. Terran) (14 커맨드) — 🟡 Medium · Type: Opening · Popularized: Flash
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Supply Depot|9|—|
|2|Command Center (nat)|14 (fantasy variant: 13)|—|
|3|Barracks|15|—|
|4|Refinery|16|—|
|5|Supply Depot|16|—|
|6|Factory|21|—|
|7|Bunker|23|—|
|8|Expansion Refinery|26|—|

Greediest standard TvT economic opener. **Source:** [14 CC (vs. Terran)](https://liquipedia.net/starcraft/14_CC_(vs._Terran)).

---
---

# PROTOSS — 12 openings

## PvZ

### P1 · Forge Fast Expand / FFE (생더블) — 🟢 High · Type: Opening · Popularized: Nal_rA, Bisu
Four scout-dependent variants (all begin: Pylon @8 at nat + scout, Forge @11):

| Step | vs 9 Pool | vs 12 Pool | vs 12 Hatch | vs Overpool |
|---|---|---|---|---|
|Cannon(s)|2× @13|@15|@18|@13|
|Nexus|@15|@15|@15|@13|
|Gateway|@15|@15|@17|@15|
|Pylon|@16|@16|@16|@16|
|Assimilator|@17|@17|@19|@17|
|Cybernetics Core|@18|@18|@20|@18|
|Zealot|@20|@19|@22|@20|

Universal PvZ opener → Stargate/Corsair. **Source:** [Forge FE (vs. Zerg)](https://liquipedia.net/starcraft/Forge_FE_(vs._Zerg)). Recent: [Snow vs Soma, ASL S20 final, Oct 2025](https://www.youtube.com/watch?v=xk3PN5zd8J0). **No sourced clock — supply only.**

### P2 · +1 Sair/Speedlot (네오 비수류) — 🟢 High · Type: Opening (off FFE) · ⏱ has clock anchor
| # | Building / upgrade | Trigger | Clock |
|---|---|---|---|
|1|(Forge FE base)|—|—|
|2|Stargate (continuous Corsairs)|—|—|
|3|+1 Ground Weapons (Forge)|—|—|
|4|Citadel of Adun + Zealot Speed|—|—|
|5|Robotics Facility + Singularity Charge|—|**"at around 9:00"**|

Target ~6 Zealots + 5 Corsairs at +1 completion. **Source:** [+1 Sair/Speedlot (vs. Zerg)](https://liquipedia.net/starcraft/%2B1_Sair/Speedlot_(vs._Zerg)). *Supply table not published; sequence + the 9:00 anchor are sourced.*

### P3 · +1 Speedzealot / 14 Nexus FE (+1 질럿) — 🟡 Medium · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Pylon (nat)|8|—|
|2|Forge|10|—|
|3|Nexus|14|—|
|4|Photon Cannon|14|—|
|5|Gateway (nat)|16|—|
|6|Assimilator (main)|16|—|
|7|Cybernetics Core|21|—|
|8|Pylon (main)|24|—|
|9|Citadel of Adun|27|—|
|10|+2 Gateways → 2nd Assimilator → Templar Archives|—|—|

+1 attack & zealot-speed pressure vs FFE follow-ups. **Source:** [+1 Speedzealot (vs Zerg)](https://liquipedia.net/starcraft/%2B1_Speedzealot_(vs_Zerg)).

### P4 · Corsair/Reaver (커세어 리버) — 🟡 Medium · Type: Opening (flexible) · Popularized: Nal_rA
Liquipedia states this "describes a strategy rather than a fixed Build Order." Rough building sequence:

| # | Building | Clock |
|---|---|---|
|1|Stargate|—|
|2|First Corsair (scout)|—|
|3|Robotics Facility|—|
|4|Early Second Gas|—|
|5|Shuttle → Reaver|—|
|6|+1 Air Weapons → Shuttle Speed|—|
|7|Second Stargate (optional) → Fleet Beacon|—|

Corsair air-control + Reaver-drop harass; long macro maps. **Source:** [Corsair/Reaver (vs. Zerg)](https://liquipedia.net/starcraft/Corsair/Reaver_(vs._Zerg)). *No supply/clock — sequence only.*

### P5 · 2 Gateway (투게이트 질럿) — 🟡 Medium · Type: Opening
Aggressive (9/9 Proxy): Probe @7, Pylon @8, Gateway @9, Gateway @9, Zealot @11, Pylon @13, Zealot @13, Zealot @15.
Safer (10/12): Pylon @8, Gateway @10, Gateway @12, Zealot @13, Pylon @15, Zealot @17, Zealot @19, Pylon @21.

Forces Zerg to spend larvae/drones on defense. **Source:** [2 Gateway (vs. Zerg)](https://liquipedia.net/starcraft/2_Gateway_(vs._Zerg)).

### P6 · Bisu Build (Corsair/DT) (커세어 다크) — 🟠 Legacy/Situational · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Pylon|8|—|
|2|Nexus|12|—|
|3|Forge|13|—|
|4|Pylon|15|—|
|5|Gateway|17|—|
|6|Photon Cannon|18|—|
|7|Assimilator|20|—|
|8|Cybernetics Core|23|—|
|9|Assimilator (nat)|~31|—|
|10|Stargate (Corsairs) → Citadel → Templar Archives → 2 DT|—|—|

The original Bisu build. **Source:** [Bisu Build](https://liquipedia.net/starcraft/Bisu_Build) (introduced vs sAviOr, MSL 2007). **Now largely outdated** vs 3-base-spire→5-hatch-hydra — kept for completeness, not as a current default.

### P15 · 4 Gate 2 Archon / 18 Nexus FE (4게이트 2아콘) — 🟡 Medium · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Pylon (nat)|8|—|
|2|Forge|10|—|
|3|2 Photon Cannons|13|—|
|4|Pylon (main)|15|—|
|5|Nexus|18|—|
|6|Gateway (nat)|18|—|
|7|Assimilator (main)|20|—|
|8|Cybernetics Core|22|—|
|9|Assimilator|25/26|—|
|10|Stargate → Citadel (@100 gas) → 3 Gateways → Templar Archives (@200 gas) → 2 Archons|—|—|

Zealot/Archon timing vs Hydra-heavy Zerg. **Source:** [4 Gate 2 Archon](https://liquipedia.net/starcraft/4_Gate_2_Archon).

## PvT

### P7 · 1 Gate Core (vs. Terran) (원게이트 코어) — 🟢 High · Type: Opening
| Step | No Zealot | One Zealot | Two Zealots |
|---|---|---|---|
|Pylon|8|8|8|
|Gateway|10|10|10|
|Assimilator|12|12|12|
|Zealot|—|13|13 / 17|
|Pylon|—|16|16|
|Cybernetics Core|13|18|20|
|Dragoon|@100% Core|@100% Core|@100% Core|

Universal PvT opener. **Source:** [1 Gate Core (vs. Terran)](https://liquipedia.net/starcraft/1_Gate_Core_(vs._Terran)). Recent: [Bisu vs TY, ASL S20, Aug 2025](https://sc2casts.com/cast36579-Bisu-vs-TY-Best-of-1-2025-ASL-Season-20-Group-Stage).

### P9 · 1 Gate Reaver (vs. Terran) (원게이트 리버) — 🟢 High · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Pylon|8|—|
|2|Gateway|10|—|
|3|Assimilator|11|—|
|4|Cybernetics Core|13|—|
|5|Pylon|15|—|
|6|(Dragoon)|18|—|
|7|(Dragoon Range)|20|—|
|8|Pylon|21|—|
|9|Robotics Facility|26 (@200 gas)|—|
|10|Pylon|29|—|
|11|Observatory|34|—|
|12|Nexus (nat)|37|—|

Reaver-shuttle harass into expand. Note: "Range + Robo finish ~33–34 supply." **Source:** [1 Gate Reaver](https://liquipedia.net/starcraft/1_Gate_Reaver).

### P13 · 2 Gate DT (투게이트 다크) — 🟡 Medium · Type: Opening (PvT)
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Pylon|8|—|
|2|Gateway|10|—|
|3|Assimilator|12|—|
|4|Cybernetics Core|14|—|
|5|Pylon|15|—|
|6|Citadel of Adun → 2nd Gateway → Templar Archives → Pylon|—|—|
|7|2 Dark Templars|@100% Templar Archives|first DT "around the six minute mark"|

Cloaked harass vs Siege-FE Terran lacking detection. **Source:** [2 Gate DT](https://liquipedia.net/starcraft/2_Gate_DT) (Anytime vs iloveoov, 2005). The "~6 min first DT" note is cross-referenced from the [2 Gate Reaver page](https://liquipedia.net/starcraft/2_Gate_Reaver_(vs._Protoss)).

## PvP

### P11 · 1 Gate Zealot/Core (원게이트 코어) — 🟢 High · Type: Opening
| Step | Dragoon First | One Zealot | zzcore (2 Zealot) |
|---|---|---|---|
|Pylon|8|8|8|
|Gateway|10|10|10|
|Assimilator|12|12|16|
|Zealot|—|13|13 / 17|
|Pylon|15|16|12|
|Cybernetics Core|13|17|20|
|Dragoon|17|@100% Core|@100% Core|

The flexible PvP backbone. **Source:** [1 Gate Core (vs. Protoss)](https://liquipedia.net/starcraft/1_Gate_Core_(vs._Protoss)). Recent: [Snow vs Rain, ASL 19 Ro8](https://www.youtube.com/watch?v=hVMCJKtC4a8).

### P12 · 2 Gate Reaver (투게이트 리버) — 🟡 Medium · Type: Opening (PvP) · ⏱ rich clock anchors
"Gate Robo Gate" variant:

| # | Building | Supply | Clock |
|---|---|---|---|
|1|Pylon|8|—|
|2|Gateway|10|—|
|3|Assimilator|11|—|
|4|Cybernetics Core|13|—|
|5|Pylon|16|—|
|6|(Dragoon Range)|21|—|
|7|Pylon|24|—|
|8|Robotics Facility|25|—|
|9|Gateway #2|29|—|
|10|Pylon|33|—|
|11|Observatory|@100% Robo|—|
|12|Observer|@100% Observatory|**~5:30**|
|13|Robotics Support Bay|40|—|
|14|Reaver ×2|41 / @100% Reaver|—|
|15|Shuttle|@100% 2nd Reaver|—|
|16|Nexus (nat)|~65|**push ~7:50–8:00**|

Other variants (sourced clocks): *Robotics-before-Range* — Range ~7:00, Observer ~6:20, Reaver+Shuttle ~8:00. *Range-before-Robo* — Range ~5:40, Observer ~6:40, Reaver+Shuttle ~7:30, Nexus ~8:40. **Source:** [2 Gate Reaver (vs. Protoss)](https://liquipedia.net/starcraft/2_Gate_Reaver_(vs._Protoss)).

---
---

# ZERG — 13 openings

## ZvP

### Z1 · 3 Base Spire into 5 Hatch Hydra (3해처리 스파이어 → 5해처리 히드라) — 🟢 High · Type: Opening · Creator: GGPlay · Popularized: Jaedong
Any standard opening converges after the 3rd Hatchery at the 2nd expo (assumes Protoss FFE):

| # | Building | Supply / trigger | Clock |
|---|---|---|---|
|1|x2–6 Zerglings|@100% Pool|—|
|2|Lair|@100 gas|—|
|3|Metabolic Boost|@100 gas|—|
|4|Spire|@100% Lair|—|
|5|Hatchery (3rd base)|32|—|
|6|Extractor (nat)|31|—|
|7|Hydralisk Den + Hatchery|35|—|
|8|Muscular Augments, Carapace, Evolution Chamber|@100% Den|—|
|9|2 Scourge|@100% Spire|—|
|10|+1 Missile Attacks + 2 Sunken Colonies|@100% Evo|—|
|11|Begin Hydralisk production|42|*"9 min mark → 85 supply" (army check)*|
|12|Extractor (3rd)|54–60|—|
|13|9–11 Mutalisks, Lurker Aspect, 2 Evo, 4th-base Hatchery|110|—|

The standard modern ZvP. **Source:** [3 Base Spire into 5 Hatch Hydra (vs. Protoss)](https://liquipedia.net/starcraft/3_Base_Spire_into_5_Hatch_Hydra_(vs._Protoss)). Soma's ASL S20 title run: [final vs Snow](https://www.youtube.com/watch?v=LqeJOt4-dds); [QF vs Best](https://www.youtube.com/watch?v=upGa1UH9kIE).

### Z2 · 12 Hatch (vs. Protoss) (12해처리) — 🟢 High · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Overlord|9|—|
|2|Hatchery (nat)|12|—|
|3|Spawning Pool|11|—|

The economical ZvP opener (branches into Z1 etc.). **Source:** [12 Hatch (vs. Protoss)](https://liquipedia.net/starcraft/12_Hatch_(vs._Protoss)).

### Z3 · 5 Hatch before Gas (vs. Protoss) (5해처리 노개스) — 🟡 Medium · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Overlord|9|—|
|2|Spawning Pool|9|—|
|3|Hatchery (scout)|11|—|
|4|Hatchery (2nd nat)|13|—|
|5|Overlord|17|—|
|6|Hatchery (nat simcity)|18|—|
|7|Extractor|26|—|
|8|Extractor|25|—|
|9|Hatchery (simcity)|27|—|
|10|Hydralisk Den|@50 gas|—|
|11|Lair|@100 gas|—|

Max-economy hydra build, no early tech. **Source:** [5 Hatch before Gas (vs. Protoss)](https://liquipedia.net/starcraft/5_Hatch_before_Gas_(vs._Protoss)).

### Z4 · 2 Hatch Hydra (vs. Protoss) (투해처리 히드라) — 🟡 Medium · Type: Opening
| # | Building | Supply / trigger | Clock |
|---|---|---|---|
|1|(flexible opening 9 Pool–12 Hatch)|—|—|
|2|Extractor|after nat Hatchery|—|
|3|Hydralisk Den|@50 gas|—|
|4|Overlord|16|—|
|5|Hydralisk Range|@150 gas|—|
|6|Hydralisk Speed|@next 150 gas|—|

Hydra pressure / bust vs cannon-light FFE. **Source:** [2 Hatch Hydra (vs. Protoss)](https://liquipedia.net/starcraft/2_Hatch_Hydra_(vs._Protoss)).

### Z6 · 2 Hatch Lurker Drop (vs. Protoss) (투해처리 럴커 드랍) — 🟠 Situational · Type: Opening
9-Overpool variant:

| # | Building | Supply / trigger | Clock |
|---|---|---|---|
|1|Overlord|9|—|
|2|Spawning Pool|9|—|
|3|Extractor|9|—|
|4|2nd Hatchery (nat)|14|—|
|5|Lair|15|—|
|6|Overlord|15|—|
|7|Drop Upgrade + Hydralisk Den|@100% Lair|—|
|8|Lurker Aspect|@100% Den|—|
|9|3rd Hatchery + 2nd gas|29|—|
|10|4th Hatchery (nat)|32|—|

All-in vs under-defended FFE. **Source:** [2 Hatch Lurker Drop (vs. Protoss)](https://liquipedia.net/starcraft/2_Hatch_Lurker_Drop_(vs._Protoss)).

## ZvT

### Z7 · 12 Hatch (vs. Terran) (12해처리) — 🟢 High · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Overlord|9|—|
|2|Hatchery (nat)|12|—|
|3|Spawning Pool|11 (or 13)|13-Pool "delays pool ~10–12 seconds"|

Standard ZvT economic opener. **Source:** [12 Hatch (vs. Terran)](https://liquipedia.net/starcraft/12_Hatch_(vs._Terran)).

### Z8 · 3 Hatch Muta (vs. Terran) (3해처리 뮤탈) — 🟢 High · Type: Opening · Popularized: sAviOr · ⏱ has clock anchor
| # | Building | Supply / trigger | Clock |
|---|---|---|---|
|1|Overlord|9|—|
|2|Hatchery (expo)|12|—|
|3|Spawning Pool|11–13|—|
|4|Hatchery|13–14|—|
|5|Extractor|12–14|—|
|6|2 Zerglings|@100% Pool|—|
|7|Overlord|16|—|
|8|Lair|@100 gas|—|
|9|Extractor|21–23|—|
|10|Zergling Speed|@next 100 gas|—|
|11|Spire|@100% Lair|**"between 4:48-4:52"**|
|12|Overlord|24|—|
|13|Expansion or Sunken Colonies|33|—|
|14|3 Overlords|@50% Spire|—|

Workhorse ZvT muta build. **Source:** [3 Hatch Muta (vs. Terran)](https://liquipedia.net/starcraft/3_Hatch_Muta_(vs._Terran)). Recent: [Soma vs Flash, ASL S21 final](https://www.youtube.com/watch?v=L7e9al312GM).

### Z9 · 2 Hatch Muta (vs. Terran) (투해처리 뮤탈) — 🟡 Medium · Type: Opening · ⏱ has clock anchor (unit)
12 Hatch variant: Overlord @9, Hatchery @12, Pool @11, Extractor @10, Zerglings @100% Pool, Lair @150 min/100 gas.
12 Pool variant: Pool @12, Extractor @11, Hatchery @11, Lair @150 min/100 gas.
Common: Overlord @16, Speed @100 min/gas, Spire @100% Lair, Mutalisks to 11.

| Milestone | Clock |
|---|---|
|Mutalisks complete (12 Hatch)|**"around 6:30"**|
|Mutalisks complete (12 Pool)|**"around 6:00"**|

Faster muta when a 3rd is hard to hold. **Source:** [2 Hatch Muta (vs. Terran)](https://liquipedia.net/starcraft/2_Hatch_Muta_(vs._Terran)). *Clocks are Mutalisk-finish, not building.*

### Z10 · 3 Hatch Lurker (vs. Terran) (3해처리 럴커) — 🟢 High · Type: Opening
| # | Building | Supply / trigger | Clock |
|---|---|---|---|
|1|Overlord|9|—|
|2|Hatchery (nat)|12|—|
|3|Spawning Pool|11–13|—|
|4|Hatchery|13|—|
|5|2–6 Zerglings|@100% Pool|—|
|6|Overlord|16|—|
|7|Extractor|16|—|
|8|Lair|@100 gas|—|
|9|Zergling Speed|@100 gas|—|
|10|Hydralisk Den|@60% Lair|—|
|11|Expansion Extractor|@90% Lair|—|
|12|Lurker Aspect + Evolution Chamber|@100% Lair|—|
|13|+1 Carapace|@100% Evo|—|

Defensive fast-tech vs +1 5-Rax. **Source:** [3 Hatch Lurker (vs. Terran)](https://liquipedia.net/starcraft/3_Hatch_Lurker_(vs._Terran)).

## ZvZ

### Z12 · 9 Pool Speed into 1 Hatch Spire (9풀 스피드 → 1해처리 스파이어) — 🟢 High · Type: Opening
| # | Building | Supply / trigger | Clock |
|---|---|---|---|
|1|Spawning Pool|9|—|
|2|Extractor|9|—|
|3|Overlord|8|—|
|4|3 Drones to gas|@100% Extractor|—|
|5|6 Zerglings|@100% Pool|—|
|6|Zergling Speed|@1st 100 gas|—|
|7|Lair|@2nd 100 gas|—|
|8|Overlord|16/17|—|
|9|Spire|@100% Lair|—|

Matchup-defining 1-base muta build. **Source:** [9 Pool Speed into 1 Hatch Spire (vs. Zerg)](https://liquipedia.net/starcraft/9_Pool_Speed_into_1_Hatch_Spire_(vs._Zerg)).

### Z13 · Overpool (vs. Zerg) (오버풀) — 🟢 High · Type: Opening
| # | Building | Supply | Clock |
|---|---|---|---|
|1|Overlord|9|—|
|2|Spawning Pool|9|—|
|3|Extractor|9|—|
|4|6 Zerglings|10|—|

(Gas + Pool finish together.) The "perfect" safe ZvZ default → Lair @100 gas → Spire @100% Lair → 3 Mutas. **Source:** [Overpool (vs. Zerg)](https://liquipedia.net/starcraft/Overpool_(vs._Zerg)).

### Z14 · 9 Gas 9 Pool (vs. Zerg) (9개스 9풀) — 🟡 Medium · Type: Opening
| # | Building | Supply / trigger | Clock |
|---|---|---|---|
|1|Overlord|9|—|
|2|Extractor (@50 min)|9|—|
|3|Spawning Pool|9|—|
|4|Lair + Zergling Speed|11|—|
|5|Spire|@100% Lair|—|
|6|Overlord|17|—|
|7|3 Mutalisks|@100% Spire|—|

Gas-first; hard-counters 9 Pool Speed. **Source:** [9 Gas, 9 Pool (vs. Zerg)](https://liquipedia.net/starcraft/9_Gas,_9_Pool_(vs._Zerg)).

### Z15 · 12 Pool (vs. Zerg) (12풀) — 🟡 Medium · Type: Opening · ⏱ has clock anchor
| # | Building | Supply / trigger | Clock |
|---|---|---|---|
|1|Overlord|9|—|
|2|Spawning Pool|12|—|
|3|Extractor|11|—|
|4|Drone|10|—|
|5|Hatchery|@next 300 min|—|
|6|Lair|@next 100 gas|**"around 3 minutes into the game"**|
|7|Zergling Speed|@next 100 gas (after ≥8 lings)|—|
|8|Spire|@100% Lair|—|
|9|Overlord|16|—|

Drone-greedy ZvZ. **Source:** [12 Pool (vs. Zerg)](https://liquipedia.net/starcraft/12_Pool_(vs._Zerg)) / [Zerg vs. Zerg Guide](https://liquipedia.net/starcraft/Zerg_vs._Zerg_Guide). *Notable: Soma closed ASL21 G7 with aggressive 12-Pool + drone harass — [recap](https://www.esportsheaven.com/features/soma-claims-asl-season-21-title-in-epic-4-3-thriller-against-legend-flash/).*

---
---

## What changed from v1 (per your feedback)

- **Timing data added** — building-centric tables with supply + verbatim sourced clock anchors. Honest headline: **BW pages are supply-keyed; only ~7 builds publish real m:ss times** (consolidated above).
- **Disjoint & specific** — combined "12 Hatch / Overpool (into the above)" split: `Z2` is now standalone **12 Hatch (vs. Protoss)**; **Overpool** is its own ZvZ entry (`Z13`). No "into the above" framing.
- **Removed non-openings** — dropped Carrier/Arbiter late-game (P10), SK Terran late-game (T10), 3-Base Hive Lurker/Defiler transition (Z11), the 2-Factory adaptation (old T5), and Light's Fast Arbiter (no sourced build steps). Also dropped "5 Hatch Hydra into Muta (vP)" — its Liquipedia article returns empty / is not a populated page, so it failed the trust bar.
- **Corrected** — "2 Gate DT" is a **PvT** build (was mislabelled PvP).

## Methodology & honesty ledger

- **No fabricated timings.** Every clock value is quoted verbatim from the cited Liquipedia page; everything else is supply/resource/build-progress as the source states it.
- **Rendering note.** Many Liquipedia BO tables are templated and don't appear via plain HTTP fetch; these were read with a JavaScript renderer, which is how the clock anchors (e.g. 5 Factory Goliaths' "~6:30 / ~7:30", 2 Gate Reaver's "~5:30") were recovered.
- **No SC2.** Every unit/building is Brood War.
- **For true second-level matching:** derive empirical per-building median timestamps from your own labelled replay corpus. See `building_events.csv` for the schema and the supply/sequence priors to seed it.
