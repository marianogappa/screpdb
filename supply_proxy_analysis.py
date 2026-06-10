import sqlite3, statistics as st, sys
from collections import defaultdict

DB = sys.argv[1] if len(sys.argv) > 1 else "screp.db"
PROV = {
    ("Build", "Supply Depot"): 8, ("Build", "Pylon"): 8, ("Build", "Hatchery"): 1,
    ("Build", "Command Center"): 10, ("Build", "Nexus"): 9, ("Unit Morph", "Overlord"): 8,
}
SEED = {"Terran": 10, "Protoss": 9, "Zerg": 9}
SUPPLY_CAP = 200
GAP_THRESHOLD = 25
MIN_ADDS = 4
MIN_GAMES = 10           # players with >= this many eligible games for per-player stats
METRICS = ["mean_gap", "max_gap", "p90_gap", "gap_cv", "supply_rate", "n_gaps_over"]

con = sqlite3.connect(DB)
con.row_factory = sqlite3.Row
players = {r["id"]: dict(r) for r in con.execute(
    "SELECT p.id, p.name, p.race, p.eapm, p.apm, p.is_winner, "
    "r.duration_seconds dur, r.matchup, p.replay_id rid "
    "FROM players p JOIN replays r ON r.id=p.replay_id WHERE p.type='Human'")}
events = defaultdict(list)
for r in con.execute("SELECT player_id, action_type, unit_type, seconds_from_game_start sec FROM commands"):
    v = PROV.get((r["action_type"], r["unit_type"]))
    if v: events[r["player_id"]].append((r["sec"], v))

def metrics(pid, p):
    evs = sorted(events.get(pid, []))
    seq = sorted([(0, SEED.get(p["race"], 0))] + [(max(0, s), v) for s, v in evs])
    end = int(0.8 * p["dur"]); times, cum = [], 0
    for s, v in seq:
        cum += v
        if s > end: break
        times.append(s)
        if cum >= SUPPLY_CAP: break
    if len(times) < MIN_ADDS: return None
    win = times[-1] - times[0]
    if win < 120: return None
    gaps = [times[i+1]-times[i] for i in range(len(times)-1)]
    total = sum(v for _, v in seq[:len(times)])
    return dict(mean_gap=st.mean(gaps), max_gap=max(gaps), p90_gap=sorted(gaps)[int(0.9*(len(gaps)-1))],
        gap_cv=(st.pstdev(gaps)/st.mean(gaps)) if st.mean(gaps) else 0, supply_rate=total/(win/60.0),
        n_gaps_over=sum(1 for g in gaps if g >= GAP_THRESHOLD),
        race=p["race"], matchup=p["matchup"], name=p["name"], eapm=p["eapm"],
        is_winner=p["is_winner"], rid=p["rid"])

rows = [m for pid, p in players.items() if (m := metrics(pid, p))]
print(f"DB={DB}  eligible player-games: {len(rows)} / {len(players)} human rows\n")

def eta2(groups):
    groups = {k: v for k, v in groups.items() if len(v) >= 3}
    allv = [v for g in groups.values() for v in g]
    if len(groups) < 2 or len(allv) < 3: return None
    gm = st.mean(allv); sst = sum((x-gm)**2 for x in allv)
    ssb = sum(len(g)*(st.mean(g)-gm)**2 for g in groups.values())
    return ssb/sst if sst else 0

def pearson(xs, ys):
    if len(xs) < 3: return 0
    mx=st.mean(xs); my=st.mean(ys)
    cov=sum((x-mx)*(y-my) for x,y in zip(xs,ys))
    dx=sum((x-mx)**2 for x in xs)**.5; dy=sum((y-my)**2 for y in ys)**.5
    return cov/(dx*dy) if dx and dy else 0

# matchup z-normalized copies of each metric
mu_stats = {}
for m in METRICS:
    by = defaultdict(list)
    for r in rows: by[r["matchup"]].append(r[m])
    mu_stats[m] = {k: (st.mean(v), st.pstdev(v) or 1) for k, v in by.items()}
for r in rows:
    for m in METRICS:
        mean, sd = mu_stats[m][r["matchup"]]
        r[m+"_z"] = (r[m]-mean)/sd

print("=== RACE / MATCHUP divergence (eta2 = variance explained) ===")
for m in METRICS:
    er = eta2({k:[r[m] for r in rows if r["race"]==k] for k in set(r["race"] for r in rows)})
    em = eta2({k:[r[m] for r in rows if r["matchup"]==k] for k in set(r["matchup"] for r in rows)})
    print(f"  {m:12s} race_eta2={er:.3f}  matchup_eta2={em:.3f}")
print("\n  matchup means (mean_gap / supply_rate / n_gaps_over):")
for mu in sorted(set(r["matchup"] for r in rows)):
    g=[r for r in rows if r["matchup"]==mu]
    print(f"    {mu}: n={len(g):4d}  mean_gap={st.mean([x['mean_gap'] for x in g]):5.1f}  rate={st.mean([x['supply_rate'] for x in g]):5.1f}  blocks={st.mean([x['n_gaps_over'] for x in g]):4.1f}")

# --- per-player: is it a trait? (players with >= MIN_GAMES) ---
byname = defaultdict(list)
for r in rows: byname[r["name"]].append(r)
heavy = {n: g for n, g in byname.items() if len(g) >= MIN_GAMES}
print(f"\n=== PER-PLAYER trait test ({len(heavy)} players with >= {MIN_GAMES} games) ===")
print("  player_eta2 = fraction of variance explained by player identity (higher = more trait-like)")
print("  split_half  = corr of player means across two halves of their games (reliability)")
for m in METRICS:
    raw_groups = {n: [r[m] for r in g] for n, g in heavy.items()}
    z_groups   = {n: [r[m+"_z"] for r in g] for n, g in heavy.items()}
    pe_raw = eta2(raw_groups); pe_z = eta2(z_groups)
    # split-half on matchup-normalized metric, deterministic even/odd by rid order
    a, b = [], []
    for n, g in heavy.items():
        gs = sorted(g, key=lambda r: r["rid"])
        ha = [r[m+"_z"] for i, r in enumerate(gs) if i % 2 == 0]
        hb = [r[m+"_z"] for i, r in enumerate(gs) if i % 2 == 1]
        if ha and hb: a.append(st.mean(ha)); b.append(st.mean(hb))
    sh = pearson(a, b)
    print(f"  {m:12s} player_eta2(raw)={pe_raw:.3f}  player_eta2(mu-norm)={pe_z:.3f}  split_half(mu-norm)={sh:+.2f}")

# --- validity ---
print("\n=== VALIDITY (matchup-normalized metrics) ===")
ea=[r["eapm"] for r in rows]
print("  corr with eAPM:")
for m in METRICS:
    print(f"    {m:12s} raw={pearson(ea,[r[m] for r in rows]):+.2f}  mu-norm={pearson(ea,[r[m+'_z'] for r in rows]):+.2f}")
print("  winners vs losers (mu-normalized mean, want != 0):")
for m in METRICS:
    w=[r[m+"_z"] for r in rows if r["is_winner"]]; l=[r[m+"_z"] for r in rows if not r["is_winner"]]
    print(f"    {m:12s} win={st.mean(w):+.2f}  lose={st.mean(l):+.2f}  diff={st.mean(w)-st.mean(l):+.2f}")
