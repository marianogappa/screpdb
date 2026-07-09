# Changelog

## [1.23.1](https://github.com/marianogappa/screpdb/compare/v1.23.0...v1.23.1) (2026-07-09)


### Bug Fixes

* **gui:** open dashboard browser from Medium launcher, add tray item ([#288](https://github.com/marianogappa/screpdb/issues/288)) ([c3cf34c](https://github.com/marianogappa/screpdb/commit/c3cf34cae24a32e186702fb8ced2075df952282b))

## [1.23.0](https://github.com/marianogappa/screpdb/compare/v1.22.0...v1.23.0) (2026-07-05)


### Features

* **ci:** auto-merge PRs labeled 'automerge' once checks pass ([#283](https://github.com/marianogappa/screpdb/issues/283)) ([12abacb](https://github.com/marianogappa/screpdb/commit/12abacbf1f8b3dc5fb4b6e272078d29f6597028e))

## [1.22.0](https://github.com/marianogappa/screpdb/compare/v1.21.1...v1.22.0) (2026-07-05)


### Features

* **release:** open Scoop bump PR as a GitHub App for hands-off auto-merge ([#281](https://github.com/marianogappa/screpdb/issues/281)) ([bf46a59](https://github.com/marianogappa/screpdb/commit/bf46a59c4a30e03206121667a34ab79a444efc15))

## [1.21.1](https://github.com/marianogappa/screpdb/compare/v1.21.0...v1.21.1) (2026-07-05)


### Bug Fixes

* **release:** run Scoop bump last so it can never strip release assets ([#277](https://github.com/marianogappa/screpdb/issues/277)) ([1cbacf8](https://github.com/marianogappa/screpdb/commit/1cbacf884e4785d4bfd9de2ddae82b9976c94749))

## [1.21.0](https://github.com/marianogappa/screpdb/compare/v1.20.0...v1.21.0) (2026-07-05)


### Features

* fix Zerg gas-trick opener undercount and curate round-13 build orders ([#274](https://github.com/marianogappa/screpdb/issues/274)) ([195d525](https://github.com/marianogappa/screpdb/commit/195d5257a47c47a5956f9264e12f934023f5b392))


### Bug Fixes

* **scoop:** unstick manifest at v1.20.0 and bump via PR, not blocked push ([#276](https://github.com/marianogappa/screpdb/issues/276)) ([dee8f85](https://github.com/marianogappa/screpdb/commit/dee8f85888297d576413a193781694fd4bd8b659))

## [1.20.0](https://github.com/marianogappa/screpdb/compare/v1.19.0...v1.20.0) (2026-07-04)


### Features

* copyable upgrade command, dismissable update banner, and self-update placement docs ([#266](https://github.com/marianogappa/screpdb/issues/266)) ([d1d0872](https://github.com/marianogappa/screpdb/commit/d1d08720c310c16d52d9a21919cd31668b7f3f7d))
* exempt catch-all residual openers from the beta tag ([#267](https://github.com/marianogappa/screpdb/issues/267)) ([35228fc](https://github.com/marianogappa/screpdb/commit/35228fcd729c659e7a0cef40007cc7d64175d81b))
* modernize MCP server for natural-language DB queries and add headless API mode ([#271](https://github.com/marianogappa/screpdb/issues/271)) ([32e0732](https://github.com/marianogappa/screpdb/commit/32e0732a989061dc9efa049738d5325908f9b948))

## [1.19.0](https://github.com/marianogappa/screpdb/compare/v1.18.0...v1.19.0) (2026-07-03)


### Features

* Go 1.26.4, round-11 beta curation, and test coverage to 81% ([#263](https://github.com/marianogappa/screpdb/issues/263)) ([7368239](https://github.com/marianogappa/screpdb/commit/7368239b0715f7c9d3378e4ebef6a4d5beee1647))


### Bug Fixes

* **ci:** stop Release double-run race; make bench README push best-effort ([#262](https://github.com/marianogappa/screpdb/issues/262)) ([7e96740](https://github.com/marianogappa/screpdb/commit/7e967402b4104b4a559e9f119d9a607ccc21f134))

## [1.18.0](https://github.com/marianogappa/screpdb/compare/v1.17.1...v1.18.0) (2026-07-02)


### Features

* flag team stacking sustained to game end below the 5-min bar ([#258](https://github.com/marianogappa/screpdb/issues/258)) ([4b5f560](https://github.com/marianogappa/screpdb/commit/4b5f56073981216b8c3d4445518d5d45976766d2))


### Bug Fixes

* **builddedup:** don't drop a produced expansion CC (factory-before-expa over-count) ([#259](https://github.com/marianogappa/screpdb/issues/259)) ([fbec72b](https://github.com/marianogappa/screpdb/commit/fbec72b571ec063175e543122a7ae4a006ba1d8a)), closes [#244](https://github.com/marianogappa/screpdb/issues/244)
* **dashboard:** show the package-manager upgrade command in the update label ([#255](https://github.com/marianogappa/screpdb/issues/255)) ([8cea499](https://github.com/marianogappa/screpdb/commit/8cea499c07bdac1429dafed97a8e17afbe09fb16))
* render resolved BO/marker labels on all surfaces + ingestion-speed benchmark ([#251](https://github.com/marianogappa/screpdb/issues/251), [#249](https://github.com/marianogappa/screpdb/issues/249)) ([#256](https://github.com/marianogappa/screpdb/issues/256)) ([d1601d5](https://github.com/marianogappa/screpdb/commit/d1601d523359515f0c7957b380f79ac066ed64dc))

## [1.17.1](https://github.com/marianogappa/screpdb/compare/v1.17.0...v1.17.1) (2026-07-02)


### Bug Fixes

* **release:** populate empty Homebrew tap on first bump ([#253](https://github.com/marianogappa/screpdb/issues/253)) ([57ff4d9](https://github.com/marianogappa/screpdb/commit/57ff4d923f66afa763ed1351f344c0b1f755bf23))

## [1.17.0](https://github.com/marianogappa/screpdb/compare/v1.16.0...v1.17.0) (2026-07-02)


### Features

* **install:** free first-install UX for macOS/Linux + rename GUI asset dashboard→gui ([#252](https://github.com/marianogappa/screpdb/issues/252)) ([0cb573f](https://github.com/marianogappa/screpdb/commit/0cb573fc0c184b200e7a8a652cd110f1e31385e4))
* **markers:** N Hatch Hydra/Muta/Lurker composition markers, dynamic N ([#245](https://github.com/marianogappa/screpdb/issues/245)) ([#246](https://github.com/marianogappa/screpdb/issues/246)) ([a5167bc](https://github.com/marianogappa/screpdb/commit/a5167bcd0509842476d4741c8e2cf245f623a9a6))
* **security:** Windows Low-integrity sandbox + single app-data root ([#237](https://github.com/marianogappa/screpdb/issues/237)) ([#250](https://github.com/marianogappa/screpdb/issues/250)) ([ae3b8ad](https://github.com/marianogappa/screpdb/commit/ae3b8adc824e9b9e36c07d4b7c6c6387e9efb50e))

## [1.16.0](https://github.com/marianogappa/screpdb/compare/v1.15.0...v1.16.0) (2026-07-01)


### Features

* **markers:** curation round 10 — remove betas, dynamic N Hatch Hydra, marker fixes ([#241](https://github.com/marianogappa/screpdb/issues/241)) ([f7b119c](https://github.com/marianogappa/screpdb/commit/f7b119c3f6dd824fccdb52272ac44ef5c34e7546))


### Bug Fixes

* **markers:** 3 Hatch Hydra counts bases at hydra-production start (+30s grace); curate wraiths, muta hit-n-run, 2jd ([#243](https://github.com/marianogappa/screpdb/issues/243)) ([e6f69df](https://github.com/marianogappa/screpdb/commit/e6f69df126922e932686287c1366ff081b9dd84c))

## [1.15.0](https://github.com/marianogappa/screpdb/compare/v1.14.0...v1.15.0) (2026-06-30)


### Features

* **markers:** Terran mech taxonomy — factories-before-expa naming + Goliath/Starport/Bunker openers ([#226](https://github.com/marianogappa/screpdb/issues/226), [#227](https://github.com/marianogappa/screpdb/issues/227)) ([#238](https://github.com/marianogappa/screpdb/issues/238)) ([f220d21](https://github.com/marianogappa/screpdb/commit/f220d21e985d4e62524f779a3821da9eb69d7927))


### Bug Fixes

* **ingest:** fix nil-player panic; make panics produce crash reports without killing the dashboard ([#234](https://github.com/marianogappa/screpdb/issues/234)) ([#240](https://github.com/marianogappa/screpdb/issues/240)) ([261c13a](https://github.com/marianogappa/screpdb/commit/261c13a2e7b83d340e89559ea52779e91582f8f9))

## [1.14.0](https://github.com/marianogappa/screpdb/compare/v1.13.0...v1.14.0) (2026-06-29)


### Features

* **markers:** Zerg pool/hatch opener curation (round 8) — supply-count fix, 13 Hatch rung, 3 Hatch Muta marker, fuzzy openers ([#235](https://github.com/marianogappa/screpdb/issues/235)) ([00956ad](https://github.com/marianogappa/screpdb/commit/00956ad9c6643e31ddac1de54c5f651ee3c19960))

## [1.13.0](https://github.com/marianogappa/screpdb/compare/v1.12.2...v1.13.0) (2026-06-27)


### Features

* **markers:** Maelstrom, Crazy Zerg & Guardians markers, 1st Observer/Mine pills, proxy overlays, and a beta tag for uncurated detections ([#233](https://github.com/marianogappa/screpdb/issues/233)) ([6c35ca5](https://github.com/marianogappa/screpdb/commit/6c35ca5e367bf9578244cbca6f479792ca9816dd))
* **markers:** retire reaver/DT/sair openers; add cannon-contain BOs, timing markers & manner pylon (closes [#225](https://github.com/marianogappa/screpdb/issues/225)) ([#229](https://github.com/marianogappa/screpdb/issues/229)) ([f58fee8](https://github.com/marianogappa/screpdb/commit/f58fee88882a7b7fd374ecd03bc6319950d764a5))
* **markers:** Terran air/specialist openers — unify 2 Port Wraith (TvT+TvZ), 2 Fact before Expa, BBS proxy + proxy starport (closes [#228](https://github.com/marianogappa/screpdb/issues/228)) ([#232](https://github.com/marianogappa/screpdb/issues/232)) ([a4dc492](https://github.com/marianogappa/screpdb/commit/a4dc49202ef7bfad646ac6aa0d3319710d66038c))


### Bug Fixes

* **earlyfilter,worldstate:** stop dropping early workers and misreading proxy bunkers on money maps ([#231](https://github.com/marianogappa/screpdb/issues/231)) ([0622ce3](https://github.com/marianogappa/screpdb/commit/0622ce385aa96bf408c5f569f506d862175c4103))

## [1.12.2](https://github.com/marianogappa/screpdb/compare/v1.12.1...v1.12.2) (2026-06-26)


### Bug Fixes

* **earlyfilter:** re-admit Cyber-Core-gated builds so 1 Gate Core isn't misread ([#220](https://github.com/marianogappa/screpdb/issues/220)) ([734cc61](https://github.com/marianogappa/screpdb/commit/734cc6180e3155d98dd4be0ebae7a5ef97558d7f))

## [1.12.1](https://github.com/marianogappa/screpdb/compare/v1.12.0...v1.12.1) (2026-06-25)


### Bug Fixes

* **ingest:** example-set re-ingest, post-save refresh, column swap ([#218](https://github.com/marianogappa/screpdb/issues/218)) ([2d27462](https://github.com/marianogappa/screpdb/commit/2d274620f967bb0cb3d72bd8cfa54d78127f520a)), closes [#213](https://github.com/marianogappa/screpdb/issues/213)

## [1.12.0](https://github.com/marianogappa/screpdb/compare/v1.11.0...v1.12.0) (2026-06-25)


### Features

* in-binary self-update from GitHub Releases ([#212](https://github.com/marianogappa/screpdb/issues/212)) ([#216](https://github.com/marianogappa/screpdb/issues/216)) ([ebc28c3](https://github.com/marianogappa/screpdb/commit/ebc28c3094849bcf57903c0f33a8edef6bfd4301))

## [1.11.0](https://github.com/marianogappa/screpdb/compare/v1.10.0...v1.11.0) (2026-06-25)


### Features

* **markers:** base-count bio (1-Base/2-Base), reaver expand modifier, reliable building counts ([#214](https://github.com/marianogappa/screpdb/issues/214)) ([fb81479](https://github.com/marianogappa/screpdb/commit/fb81479b54b5b355b159da8533298047f648d407))

## [1.10.0](https://github.com/marianogappa/screpdb/compare/v1.9.0...v1.10.0) (2026-06-24)


### Features

* **markers:** build-order modifiers, curated Zerg goldens, and opener accuracy fixes ([#210](https://github.com/marianogappa/screpdb/issues/210)) ([31ccca9](https://github.com/marianogappa/screpdb/commit/31ccca962f8345c7fedc018add9c75bc067ea1d7))

## [1.9.0](https://github.com/marianogappa/screpdb/compare/v1.8.0...v1.9.0) (2026-06-21)


### Features

* **dashboard:** rework unit composition into phased bars + spellcasts ([#207](https://github.com/marianogappa/screpdb/issues/207)) ([a3148c6](https://github.com/marianogappa/screpdb/commit/a3148c6a8b8ed5ede17a841c1d112deffa70cf16))
* **markers:** detect Mutalisk hit-and-run harass (closes [#194](https://github.com/marianogappa/screpdb/issues/194)) ([#209](https://github.com/marianogappa/screpdb/issues/209)) ([04f858a](https://github.com/marianogappa/screpdb/commit/04f858a995e3ca098a8c2cf70411a8bff063ed68))

## [1.8.0](https://github.com/marianogappa/screpdb/compare/v1.7.0...v1.8.0) (2026-06-21)


### Features

* **onboarding:** embedded sample replay set + ingest modal rework (closes [#203](https://github.com/marianogappa/screpdb/issues/203)) ([#206](https://github.com/marianogappa/screpdb/issues/206)) ([5f5d9a4](https://github.com/marianogappa/screpdb/commit/5f5d9a482224d6f0d7432d479f7111d917222049))


### Bug Fixes

* **bo:** gate Bunker Rush on offensive spatial event, not topology alone (closes [#164](https://github.com/marianogappa/screpdb/issues/164)) ([#202](https://github.com/marianogappa/screpdb/issues/202)) ([3decce0](https://github.com/marianogappa/screpdb/commit/3decce08866989cfa4007b0388bd6f2991848283))

## [1.7.0](https://github.com/marianogappa/screpdb/compare/v1.6.1...v1.7.0) (2026-06-20)


### Features

* **attacks:** live 1v1 bilateral-fight attack detection ([#186](https://github.com/marianogappa/screpdb/issues/186)) ([#197](https://github.com/marianogappa/screpdb/issues/197)) ([3f92885](https://github.com/marianogappa/screpdb/commit/3f92885964d58544fd4a5371b9192e35aade80a4))
* **nydus:** detect offensive nydus canals as a game event and marker (closes [#193](https://github.com/marianogappa/screpdb/issues/193)) ([#201](https://github.com/marianogappa/screpdb/issues/201)) ([2f7cb87](https://github.com/marianogappa/screpdb/commit/2f7cb87ed564c07f4c80b8b254324d0036515c30))


### Bug Fixes

* **rush:** detect bunker rush on standard 1v1 maps (closes [#195](https://github.com/marianogappa/screpdb/issues/195)) ([#200](https://github.com/marianogappa/screpdb/issues/200)) ([19f9e8c](https://github.com/marianogappa/screpdb/commit/19f9e8c58e32a3b2552edf6b0c7536de28db5ebc))
* **rush:** detect zergling rush into enemy's neutral natural on standard maps (closes [#196](https://github.com/marianogappa/screpdb/issues/196)) ([#199](https://github.com/marianogappa/screpdb/issues/199)) ([7d5f0c4](https://github.com/marianogappa/screpdb/commit/7d5f0c40e7fc05262bafe2eebd23ba25f79f7601))

## [1.6.1](https://github.com/marianogappa/screpdb/compare/v1.6.0...v1.6.1) (2026-06-17)


### Bug Fixes

* build-order and cliff-drop detection accuracy + coordinate-unit normalization ([#184](https://github.com/marianogappa/screpdb/issues/184)) ([c48b221](https://github.com/marianogappa/screpdb/commit/c48b22142f2254e912d73d23fa7a9ce78d64576a))

## [1.6.0](https://github.com/marianogappa/screpdb/compare/v1.5.0...v1.6.0) (2026-06-14)


### Features

* animated game-events map, games-list polish, and Wraith filter ([#180](https://github.com/marianogappa/screpdb/issues/180)) ([45349d7](https://github.com/marianogappa/screpdb/commit/45349d7d4bf6abfab7932374ea74ccba8513bcf9))
* preferred scene-named build orders via tiered fallback ([#183](https://github.com/marianogappa/screpdb/issues/183)) ([bcca66c](https://github.com/marianogappa/screpdb/commit/bcca66c1f57fb97bf0d4a9da3df4ada8686c7e15))

## [1.5.0](https://github.com/marianogappa/screpdb/compare/v1.4.0...v1.5.0) (2026-06-13)


### Features

* enrich production/research commands with inferred coordinates (closes [#175](https://github.com/marianogappa/screpdb/issues/175)) ([#178](https://github.com/marianogappa/screpdb/issues/178)) ([0dc3326](https://github.com/marianogappa/screpdb/commit/0dc3326e5bc9c3b1637e26384ac5acb9d1765860))

## [1.4.0](https://github.com/marianogappa/screpdb/compare/v1.3.0...v1.4.0) (2026-06-09)


### Features

* crash reporting, version footer & Scoop install (closes [#165](https://github.com/marianogappa/screpdb/issues/165)) ([#169](https://github.com/marianogappa/screpdb/issues/169)) ([7a0c5a2](https://github.com/marianogappa/screpdb/commit/7a0c5a2776a9b3370882d7f09378de9d9ecd53d7))
* drop first-unit efficiency aggregation, curate per-game report (closes [#166](https://github.com/marianogappa/screpdb/issues/166)) ([#168](https://github.com/marianogappa/screpdb/issues/168)) ([03eff8d](https://github.com/marianogappa/screpdb/commit/03eff8d13065bbfed3530303409bf3d11a70c758))
* UX improvements and Double Stargate marker (closes [#167](https://github.com/marianogappa/screpdb/issues/167)) ([#172](https://github.com/marianogappa/screpdb/issues/172)) ([9e8a2ab](https://github.com/marianogappa/screpdb/commit/9e8a2ab55d42d9813df3fd5e870439f85c5eacff))


### Bug Fixes

* count non-HP upgrades as researches in Never-researched marker (closes [#163](https://github.com/marianogappa/screpdb/issues/163)) ([#171](https://github.com/marianogappa/screpdb/issues/171)) ([05a57e9](https://github.com/marianogappa/screpdb/commit/05a57e967f0cb378c901eaaa541be0ad787400e4))

## [1.3.0](https://github.com/marianogappa/screpdb/compare/v1.2.0...v1.3.0) (2026-06-07)


### Features

* expert timings for composition Terran build orders (closes [#158](https://github.com/marianogappa/screpdb/issues/158)) ([#160](https://github.com/marianogappa/screpdb/issues/160)) ([1148f31](https://github.com/marianogappa/screpdb/commit/1148f31992b75fd87c419b1b32f56baeefec52c3))
* **markers:** composition-based Terran build orders (closes [#155](https://github.com/marianogappa/screpdb/issues/155)) ([#156](https://github.com/marianogappa/screpdb/issues/156)) ([10dc997](https://github.com/marianogappa/screpdb/commit/10dc997c75541ba8fd350f317343d96f0451456c))
* rework early-game event items layout & overlays (closes [#159](https://github.com/marianogappa/screpdb/issues/159)) ([#161](https://github.com/marianogappa/screpdb/issues/161)) ([684eb39](https://github.com/marianogappa/screpdb/commit/684eb393579fa02ae1fd77effad1409ea1df2798))

## [1.2.0](https://github.com/marianogappa/screpdb/compare/v1.1.2...v1.2.0) (2026-06-06)


### Features

* **dedup:** selection-tag build dedup — worker one-at-a-time + never-produced (closes [#152](https://github.com/marianogappa/screpdb/issues/152)) ([#153](https://github.com/marianogappa/screpdb/issues/153)) ([2329157](https://github.com/marianogappa/screpdb/commit/23291579d4bc8fb70ea747eaa5aa1a8f4a1df7c9))

## [1.1.2](https://github.com/marianogappa/screpdb/compare/v1.1.1...v1.1.2) (2026-06-02)


### Bug Fixes

* **dashboard:** repair broken Skill proxies tabs (closes [#147](https://github.com/marianogappa/screpdb/issues/147)) ([#148](https://github.com/marianogappa/screpdb/issues/148)) ([6390fea](https://github.com/marianogappa/screpdb/commit/6390fea86839c158601603a42cb50cc3317b1187))
* **events:** re-snapshot teams at Finalize so allies aren't shown attacking each other (closes [#146](https://github.com/marianogappa/screpdb/issues/146)) ([#150](https://github.com/marianogappa/screpdb/issues/150)) ([b8f6f3d](https://github.com/marianogappa/screpdb/commit/b8f6f3d54ba6c997b147490fab55f8246a11527d))
* **melee:** credit winners on multi-team melee games with late alliances (closes [#130](https://github.com/marianogappa/screpdb/issues/130)) ([#151](https://github.com/marianogappa/screpdb/issues/151)) ([204f680](https://github.com/marianogappa/screpdb/commit/204f68005cf8db08da29c6eb38fe6913c1e9ccd9))

## [1.1.1](https://github.com/marianogappa/screpdb/compare/v1.1.0...v1.1.1) (2026-05-31)


### Bug Fixes

* **markers:** require same tile for Build dedup, not just a 3s window (closes [#141](https://github.com/marianogappa/screpdb/issues/141)) ([#144](https://github.com/marianogappa/screpdb/issues/144)) ([896d8d6](https://github.com/marianogappa/screpdb/commit/896d8d613e9e05f20c503e4ac269b9f6ed9c499f))

## [1.1.0](https://github.com/marianogappa/screpdb/compare/v1.0.0...v1.1.0) (2026-05-31)


### Features

* **spec:** generated, test-backed SPECIFICATION.md (closes [#138](https://github.com/marianogappa/screpdb/issues/138)) ([#142](https://github.com/marianogappa/screpdb/issues/142)) ([7a3fc66](https://github.com/marianogappa/screpdb/commit/7a3fc66404a8f8dd77183129edaecf0171b1e030))

## [1.0.0](https://github.com/marianogappa/screpdb/compare/v0.25.0...v1.0.0) (2026-05-31)


### ⚠ BREAKING CHANGES

* removed the dashboard "Ask AI" feature, the --ai-vendor/ --ai-api-key/--ai-model flags, and the ingest --watch flag.

### Features

* gate all I/O behind facades; remove AI and fswatch ([#135](https://github.com/marianogappa/screpdb/issues/135)) ([#139](https://github.com/marianogappa/screpdb/issues/139)) ([194fe17](https://github.com/marianogappa/screpdb/commit/194fe17a5a113e5f132e068d00dab5e16d037814))

## [0.25.0](https://github.com/marianogappa/screpdb/compare/v0.24.0...v0.25.0) (2026-05-30)


### Features

* **build-orders:** expand coverage to ~all player-replays + dashboard pill UX ([#136](https://github.com/marianogappa/screpdb/issues/136)) ([6f4c8d8](https://github.com/marianogappa/screpdb/commit/6f4c8d81404ebe546a38c03a5316c6d5d290ac70))

## [0.24.0](https://github.com/marianogappa/screpdb/compare/v0.23.0...v0.24.0) (2026-05-17)


### Features

* alliance event context + DT-drop classification (closes [#133](https://github.com/marianogappa/screpdb/issues/133)) ([#134](https://github.com/marianogappa/screpdb/issues/134)) ([d550ac8](https://github.com/marianogappa/screpdb/commit/d550ac893a05bb247cd9fa6523ee5ee1521975fc))
* detect Drops with source/target inference ([#131](https://github.com/marianogappa/screpdb/issues/131)) ([f4cc17a](https://github.com/marianogappa/screpdb/commit/f4cc17abcb110fb68e044ab0f7f32302aa768045))

## [0.23.0](https://github.com/marianogappa/screpdb/compare/v0.22.0...v0.23.0) (2026-05-14)


### Features

* **dashboard:** games-list polish + per-player gate fix ([#129](https://github.com/marianogappa/screpdb/issues/129)) ([b25df42](https://github.com/marianogappa/screpdb/commit/b25df42488233a092726b5a9608b805451b8c531))
* **markers:** per-matchup gate for never_researched / never_upgraded in 1v1 ([#128](https://github.com/marianogappa/screpdb/issues/128)) ([2ee369b](https://github.com/marianogappa/screpdb/commit/2ee369bf21e476743658eb38be5e068547baee12))
* redesign Alliances tab with mutual-clique detection and contextual timeline ([#124](https://github.com/marianogappa/screpdb/issues/124)) ([f723f71](https://github.com/marianogappa/screpdb/commit/f723f71d42e3240563b23a1f88a05df2a1f1c2ce))

## [0.22.0](https://github.com/marianogappa/screpdb/compare/v0.21.0...v0.22.0) (2026-05-10)


### Features

* estimate recall destination + games-list polish ([#118](https://github.com/marianogappa/screpdb/issues/118)) ([610427d](https://github.com/marianogappa/screpdb/commit/610427d43973f5b2f16c38b52dee65c2e68b51b2))

## [0.21.0](https://github.com/marianogappa/screpdb/compare/v0.20.0...v0.21.0) (2026-05-09)


### Features

* player Summary tab with matchup/format cards and outlier pills ([#116](https://github.com/marianogappa/screpdb/issues/116)) ([83ea2b7](https://github.com/marianogappa/screpdb/commit/83ea2b77052e49cd6d43346da38329bdd871e3fb))

## [0.20.0](https://github.com/marianogappa/screpdb/compare/v0.19.1...v0.20.0) (2026-05-07)


### Features

* UI polish, mid-map attack fallback, viewport-rate fix, phase boundary fix ([#112](https://github.com/marianogappa/screpdb/issues/112)) ([908ae00](https://github.com/marianogappa/screpdb/commit/908ae00c5310011dff48a98855c10f597b3f0e98))

## [0.19.1](https://github.com/marianogappa/screpdb/compare/v0.19.0...v0.19.1) (2026-05-04)


### Bug Fixes

* **dashboard:** show cliff drop pill on replay summary featuring strip ([#107](https://github.com/marianogappa/screpdb/issues/107)) ([a100c47](https://github.com/marianogappa/screpdb/commit/a100c4703fa6bea6ac0e7c3d120859740cefc4b2))

## [0.19.0](https://github.com/marianogappa/screpdb/compare/v0.18.0...v0.19.0) (2026-05-03)


### Features

* **ingest:** ~50% faster replay ingest via map-analyzer caching ([#104](https://github.com/marianogappa/screpdb/issues/104)) ([d6c2d2e](https://github.com/marianogappa/screpdb/commit/d6c2d2e6419ea208a09ce4e737324557852563b2))

## [0.18.0](https://github.com/marianogappa/screpdb/compare/v0.17.1...v0.18.0) (2026-05-03)


### Features

* 1v1 TvZ Mutalisk-Turret timing marker + tab; quiet stale-replays nag ([0ebff06](https://github.com/marianogappa/screpdb/commit/0ebff06fd5aacc99ee8f81a9401f05847722a2ff))
* alliance topology tracking, fallback team derivation, team-stacking flag ([f882eeb](https://github.com/marianogappa/screpdb/commit/f882eeb7eb1a26cf055f8088e0714dbbdc9539bb))
* batch of UX, correctness, and performance improvements ([#101](https://github.com/marianogappa/screpdb/issues/101)) ([d445d24](https://github.com/marianogappa/screpdb/commit/d445d244755e7b3fa3651be5d14ae75fb927d9e4))
* BGH cliff-drop marker, settings migration stream, replay-filter simplification ([fbcdb54](https://github.com/marianogappa/screpdb/commit/fbcdb542d77e9369e5e7333ae8821cc3349bc2c6))
* dedup research/upgrade commands using Liquipedia game knowledge ([b08f47c](https://github.com/marianogappa/screpdb/commit/b08f47c947c1639c736846bb3e5d993aa880e6bb))
* discovered T/P build orders, Money-map UX, proxy chips, rush tightening ([b149ad8](https://github.com/marianogappa/screpdb/commit/b149ad8ee7653cbebef4f3c9af12a0c3c2e7b602))
* early-game spam filter + simplified Zerg build orders ([#103](https://github.com/marianogappa/screpdb/issues/103)) ([0fa0196](https://github.com/marianogappa/screpdb/commit/0fa0196ffc1444289db574f696103085ad9e97ab))
* empty-state ingest auto-open, footer credits, live ingest list, version awareness ([be5deb2](https://github.com/marianogappa/screpdb/commit/be5deb218a0d4b86acb6a619a5f57e66cd62b3fa))
* gate 1-1-1 on Money maps; compact game-list filters ([659e896](https://github.com/marianogappa/screpdb/commit/659e896a410e5f5b411e20d42b01d1618d139941))
* ingest profiler, UMS auto-discard + map-type filter, player report additions ([785e233](https://github.com/marianogappa/screpdb/commit/785e2334c89a1b8b7374681538b56babad724535))
* migrate markers into replay_events (registry-driven pills) ([#99](https://github.com/marianogappa/screpdb/issues/99)) ([ba4de8a](https://github.com/marianogappa/screpdb/commit/ba4de8a7c588891a24e78f97a20c8d803d45d794))
* per-replay analyzer version + bulk re-analyze stale replays ([4fe5294](https://github.com/marianogappa/screpdb/commit/4fe52940e192df7e9ce8f5209f85c378c9411ba4))
* phased Game Events list with colored names, inline icons, animated overlays ([a3abce1](https://github.com/marianogappa/screpdb/commit/a3abce1b9ea8e016e25a62f357fdef1e08be1866))
* refactor player report into tabs with lazy loading ([#102](https://github.com/marianogappa/screpdb/issues/102)) ([a9677b7](https://github.com/marianogappa/screpdb/commit/a9677b71d486f96b8b4a619b63ac90ba5230ccf1))
* unified Units tab with 0-4min scaled timeline ([007c392](https://github.com/marianogappa/screpdb/commit/007c3927d5ecf123f5635a47cedeaa7b6410db9b))


### Bug Fixes

* BO events now show 'X opens with Y' and use real ownership polygons ([fa49e00](https://github.com/marianogappa/screpdb/commit/fa49e008ca217a4dc134d049327dd02d56449e30))
* erase-data / --clean now actually drops tables ([f41a5c3](https://github.com/marianogappa/screpdb/commit/f41a5c380b847fa2b662348241a3304b3547ef3f))
* faster stale-replays hint tooltip + inline Dismiss button ([7962517](https://github.com/marianogappa/screpdb/commit/7962517bfca7e20e1fff338fbeecda03547d1e5d))
* filter same-team players from drop/attack/scout/nuke events ([8dc02f0](https://github.com/marianogappa/screpdb/commit/8dc02f0c175b2e918a2c7886e96f784a3b94bf5b))
* game-report polish — taller Units timeline, BO icons, scoped overlay redraw, summary-row reshape ([c907fdd](https://github.com/marianogappa/screpdb/commit/c907fdd5c464583ba95b616bffc808b7eb259b50))
* keep ingest WS alive while modal is closed so games-list polling works ([ef685e5](https://github.com/marianogappa/screpdb/commit/ef685e5632e069b0cd08c36dfee5895ecb8ee638))
* stamp analyzer_algorithm_version on every replay; tooltip stays put ([b51226b](https://github.com/marianogappa/screpdb/commit/b51226b542d0fcca90a0f7a68fb6be1680994c2b))
* tighten game-report phases, Units timeline sizing, no-team warnings ([6b97bf8](https://github.com/marianogappa/screpdb/commit/6b97bf89665a0b5a80e82348f7ca58d6425a234c))

## [0.17.1](https://github.com/marianogappa/screpdb/compare/v0.17.0...v0.17.1) (2026-04-22)


### Bug Fixes

* harden release artifact upload checks ([662db86](https://github.com/marianogappa/screpdb/commit/662db860c47bdf336d06330c8cf8bb819f92b715))
* harden release artifact upload checks ([817a871](https://github.com/marianogappa/screpdb/commit/817a87196fc1c2136838eb80c6fe6147d71fea2c))

## [0.17.0](https://github.com/marianogappa/screpdb/compare/v0.16.0...v0.17.0) (2026-04-22)


### Features

* Implement aliases feature. ([b192b9c](https://github.com/marianogappa/screpdb/commit/b192b9c1ad2d8b2bbc5bc3a38bbafdc90e3922eb))
* Implement aliases feature. ([c65f0ad](https://github.com/marianogappa/screpdb/commit/c65f0ad2c29f70bfcf27ba12aff77475c5dd44e3))
* Implement dashboard revamp. ([3989567](https://github.com/marianogappa/screpdb/commit/39895675674b8777c1527dc96abc7618f018e80e))
* Implement dashboard revamp. ([a59938b](https://github.com/marianogappa/screpdb/commit/a59938b96d8be6313099f5a1b3784c0d8407ad34))
* Implement See Replay backend endpoint. ([69d96a5](https://github.com/marianogappa/screpdb/commit/69d96a5cef469e73c4e456b9aee67488c198713b))
* Implement See Replay backend endpoint. ([fc7a07a](https://github.com/marianogappa/screpdb/commit/fc7a07ad333c85a9c38846266782fb40241d0f49))
* Implement significant dashboard improvements. ([d2465a1](https://github.com/marianogappa/screpdb/commit/d2465a107db1d295364239f91a111a02fba907ce))
* Implement significant Game Event improvements. ([9851881](https://github.com/marianogappa/screpdb/commit/98518819011dc3e7ca23e8403c76d9582e6f9b9f))
* Implements pattern orchestrator game events logic. ([594f191](https://github.com/marianogappa/screpdb/commit/594f191c2b27137c543c95fbbc181360e9038906))
* Implements pattern orchestrator game events logic. ([79230d2](https://github.com/marianogappa/screpdb/commit/79230d28111c15e60034fcd3edd77caf086a49fd))
* Player report improvements. ([2a726dc](https://github.com/marianogappa/screpdb/commit/2a726dcbd16ba8db544ec3898447f959678c327e))
* Significantly improve Game Events report. ([c2cdd00](https://github.com/marianogappa/screpdb/commit/c2cdd0085397b13225f84bdce66bdd832ec84c51))
* Support OpenAI, Gemini & Anthropic as LLM vendors. ([3506bf4](https://github.com/marianogappa/screpdb/commit/3506bf48c193acd7a2b7d02cd32ea4b3a24fb927))
* Support OpenAI, Gemini & Anthropic as LLM vendors. ([7e5a89b](https://github.com/marianogappa/screpdb/commit/7e5a89beee06ee167fe7250c9cb664623f056e41))
* UI improvements ([b6707f5](https://github.com/marianogappa/screpdb/commit/b6707f50803527c733f6b4f7a0a2d5a0c8d3a0c5))
* Upgrade map analysis. Fix schema issue. ([84b98b5](https://github.com/marianogappa/screpdb/commit/84b98b50b543b78b7bcfe5c588e76cc0d17a798f))
* Various improvements to game reports. ([1f22998](https://github.com/marianogappa/screpdb/commit/1f22998460c31ad4ea6d1e5b897d6ab1c3edcbf9))


### Bug Fixes

* added strategyOneDriveUser() to handle Replays inside OneDrive folder ([62aca55](https://github.com/marianogappa/screpdb/commit/62aca5528e6eccbca96e79df34300c6d18e4d8a9))
* adding handler path for Users on Windows11 who have their Replays in OneDrive folder ([ec58503](https://github.com/marianogappa/screpdb/commit/ec585031dc79f0e85f536524c6ef2e69b4c9d634))
* Chat commands misassigned. Better command assignment strategy. ([d52638c](https://github.com/marianogappa/screpdb/commit/d52638cceedb34826f3f743545a365977b5f6edf))
* Chat commands misassigned. Better command assignment stratgy. ([5b0891d](https://github.com/marianogappa/screpdb/commit/5b0891de5f0e4297ce4fa77e91223f877b369ef1))
* Fix dashboard slowness, post-load widgets in parallel. ([25c9eb7](https://github.com/marianogappa/screpdb/commit/25c9eb7bdc5993ebca08cb8853f666259db1c19f))
* Fix dashboard slowness, post-load widgets in parallel. ([8e03873](https://github.com/marianogappa/screpdb/commit/8e038734cf0878ad709aa09f01acbbffccc6f66f))
* Fix Gemini requests. ([81aef9b](https://github.com/marianogappa/screpdb/commit/81aef9b110d9483ca939ae99d7593332421f740f))
* Fix Gemini requests. ([0194fc3](https://github.com/marianogappa/screpdb/commit/0194fc3006f096439492dc6f1ddbd195aad7e845))
* make map image sync resilient in CI. ([b2e8075](https://github.com/marianogappa/screpdb/commit/b2e8075e3b97bcda3f904fa4fce689ff86411481))

## [0.16.0](https://github.com/marianogappa/screpdb/compare/v0.15.1...v0.16.0) (2026-02-13)


### Features

* Implement built-in frontend. ([a0626f4](https://github.com/marianogappa/screpdb/commit/a0626f4699b9c9b31c7e3ac913466b39953b983b))
* Implement built-in frontend. ([537bd50](https://github.com/marianogappa/screpdb/commit/537bd50bbe7e50294ac702684d755775d4ab19d5))
* Implement dashboard replay filtering. ([7dcfb38](https://github.com/marianogappa/screpdb/commit/7dcfb3816d559692a22fe07d167b5d9886f63ed7))
* Implement dashboard replay filtering. ([7d423bd](https://github.com/marianogappa/screpdb/commit/7d423bd1fa4060301e9ef6cf206c1e1a82112840))

## [0.15.1](https://github.com/marianogappa/screpdb/compare/v0.15.0...v0.15.1) (2026-02-12)


### Bug Fixes

* Implement performance improvements for SQLite ingestion. ([b2235e3](https://github.com/marianogappa/screpdb/commit/b2235e32d196a81a5498b9bdfb21bf334646209c))
* Implement performance improvements for SQLite ingestion. ([a0e5405](https://github.com/marianogappa/screpdb/commit/a0e5405966a66dfea81d781a2ef1d3a2a5e1fdb9))

## [0.15.0](https://github.com/marianogappa/screpdb/compare/v0.14.2...v0.15.0) (2026-02-11)


### Features

* **clean:** separated --clean from --clean-dashboard, now --clean-dashboard only cleans dashboards and --clean only cleans everything but keeps dashboards ([afe5881](https://github.com/marianogappa/screpdb/commit/afe5881c19264903119de6268bd52e5c1d2ea3ce))
* Default to dashboard command. Trigger ingest via UI. ([04f9ac6](https://github.com/marianogappa/screpdb/commit/04f9ac669c9153a2bb71ea1a9034fdf26c89a8a2))
* Default to dashboard command. Trigger ingest via UI. ([629c1cd](https://github.com/marianogappa/screpdb/commit/629c1cdccc15c878c641fd3850a48b3832d80be0))
* Remove color and TableColumns configs. ([30347b0](https://github.com/marianogappa/screpdb/commit/30347b0483b610b283ae692c6c0b30f8ad467bb6))
* Remove colouring configuration. ([385d9b8](https://github.com/marianogappa/screpdb/commit/385d9b89679bcf1ec43dcb4d1fefe33770c60fa9))
* separated --clean from --clean-dashboard ([62e636a](https://github.com/marianogappa/screpdb/commit/62e636ac2f60b499c946b5ee676649bba01e2dd6))
* Use SQLite as database layer. ([e78a65e](https://github.com/marianogappa/screpdb/commit/e78a65e0aae90c02b70a444e481fbc3d660efa8b))
* Use SQLite as database layer. ([8446810](https://github.com/marianogappa/screpdb/commit/84468109400ca6d484431612e459d0ebb9f7a5bf))


### Bug Fixes

* Add axis labels on charts. ([fee88b2](https://github.com/marianogappa/screpdb/commit/fee88b201c5bb23b4afd46170484a5195c079c53))
* Add axis labels on chorts. ([a82f760](https://github.com/marianogappa/screpdb/commit/a82f760568fdb2980356a8eca2ef6484c7901176))
* Separate concerns and remove unuseful comments ([274421c](https://github.com/marianogappa/screpdb/commit/274421c8ea43f0ac7f83d34d06aa7439641bdac5))

## [0.14.2](https://github.com/marianogappa/screpdb/compare/v0.14.1...v0.14.2) (2026-01-04)


### Bug Fixes

* Remember chosen variable values. Fix histogram charts. ([5bf0c89](https://github.com/marianogappa/screpdb/commit/5bf0c89bdbc6c2e7475d34a890b89329cca5b768))

## [0.14.1](https://github.com/marianogappa/screpdb/compare/v0.14.0...v0.14.1) (2026-01-01)


### Bug Fixes

* Make empty commit to trigger release. ([447d26b](https://github.com/marianogappa/screpdb/commit/447d26bfb6767fdc1c2a66295899736f8c3073cb))
* Make empty commit to trigger release. ([5c0941f](https://github.com/marianogappa/screpdb/commit/5c0941f9e8b533f06bee23715d1f9c8755fa94a3))

## [0.14.0](https://github.com/marianogappa/screpdb/compare/v0.13.4...v0.14.0) (2026-01-01)


### Features

* Add seconds_from_game_start to commands. ([055b1c5](https://github.com/marianogappa/screpdb/commit/055b1c52362b4a16727b987067c545c6fb9e7757))
* Add seconds_from_game_start to commands. ([4263854](https://github.com/marianogappa/screpdb/commit/4263854fbde6ac1165c6b850be517a94d52e4674))

## [0.13.4](https://github.com/marianogappa/screpdb/compare/v0.13.3...v0.13.4) (2025-11-26)


### Bug Fixes

* Revert "fix: Build binaries for important os/archs." ([d10f607](https://github.com/marianogappa/screpdb/commit/d10f60780bab895160b57f46e7e1f98ecba574ca))

## [0.13.3](https://github.com/marianogappa/screpdb/compare/v0.13.2...v0.13.3) (2025-11-26)


### Bug Fixes

* Build binaries for important os/archs. ([295ab3d](https://github.com/marianogappa/screpdb/commit/295ab3d7f6fc23671f54559521242df0e20481c5))

## [0.13.2](https://github.com/marianogappa/screpdb/compare/v0.13.1...v0.13.2) (2025-11-26)


### Bug Fixes

* Address all deprecations on GoReleaser job. ([d719810](https://github.com/marianogappa/screpdb/commit/d719810c533eea5e5ba31e3125718ebc5d83ff7c))

## [0.13.1](https://github.com/marianogappa/screpdb/compare/v0.13.0...v0.13.1) (2025-11-26)


### Bug Fixes

* Fix checksum violation bug ([6e6fa41](https://github.com/marianogappa/screpdb/commit/6e6fa41ff23cf7540d0bf971e8d866cd7bdeb496))
* Fixes checksum-related unique key violation. ([b0ac4f4](https://github.com/marianogappa/screpdb/commit/b0ac4f476be8e85bbfbb22df19f66b53e18ee39b))
