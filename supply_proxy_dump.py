import sqlite3, statistics as st, sys, math, json
from collections import defaultdict

DB = sys.argv[1] if len(sys.argv) > 1 else "cwal.db"
PROV={("Build","Supply Depot"):8,("Build","Pylon"):8,("Build","Hatchery"):1,
      ("Build","Command Center"):10,("Build","Nexus"):9,("Unit Morph","Overlord"):8}
SEED={"Terran":10,"Protoss":9,"Zerg":9}; CAP=200; MIN_ADDS=4; MID_END=900; MIN_GAMES=10

con=sqlite3.connect(DB); con.row_factory=sqlite3.Row
players={r["id"]:dict(r) for r in con.execute(
 "SELECT p.id,p.name,p.race,p.is_winner,r.duration_seconds dur,r.matchup,r.map_name map,p.replay_id rid,r.replay_date dt "
 "FROM players p JOIN replays r ON r.id=p.replay_id WHERE p.type='Human'")}
events=defaultdict(list)
for r in con.execute("SELECT player_id,action_type,unit_type,seconds_from_game_start sec FROM commands"):
    v=PROV.get((r["action_type"],r["unit_type"]))
    if v: events[r["player_id"]].append((r["sec"],v))

w=lambda t: max(0.0,1.0-t/MID_END)  # linear15

def game(pid,p):
    evs=sorted(events.get(pid,[]))
    seq=sorted([(0,SEED.get(p["race"],0))]+[(max(0,s),v) for s,v in evs])
    end=int(0.8*p["dur"]); times,cum=[],0
    for s,v in seq:
        cum+=v
        if s>end: break
        times.append(s)
        if cum>=CAP: break
    if len(times)<MIN_ADDS or times[-1]-times[0]<120: return None
    g=[(times[i],times[i+1]-times[i]) for i in range(len(times)-1)]
    ws=[w(t) for t,_ in g]; sw=sum(ws)
    wmean=sum(a*d for a,(_,d) in zip(ws,g))/sw if sw else None
    return dict(name=p["name"],matchup=p["matchup"],race=p["race"],is_winner=p["is_winner"],
                rid=p["rid"],map=p["map"],dt=p["dt"],wmean=wmean,gaps=g)

rows=[r for pid,p in players.items() if (r:=game(pid,p)) and r["wmean"] is not None]

# matchup z-norm of wmean; lower gap = better, so goodness = -z
by=defaultdict(list)
for r in rows: by[r["matchup"]].append(r["wmean"])
stt={k:(st.mean(v),st.pstdev(v) or 1) for k,v in by.items()}
for r in rows:
    m,sd=stt[r["matchup"]]; r["good"]=-(r["wmean"]-m)/sd

byname=defaultdict(list)
for r in rows: byname[r["name"]].append(r)
heavy={n:g for n,g in byname.items() if len(g)>=MIN_GAMES}
pmean={n:st.mean([x["good"] for x in g]) for n,g in heavy.items()}
order=sorted(pmean.values())
def pct(v): return 100*sum(1 for x in order if x<=v)/len(order)

# leaderboard
lb=sorted(heavy, key=lambda n:-pmean[n])
print("=== LEADERBOARD (heavy players, supply-discipline score) ===")
for n in lb:
    print(f"  {pct(pmean[n]):5.1f}pct  {pmean[n]:+.2f}  {n[:18]:18s}  games={len(heavy[n])}")

# pick an interesting example: mid-pack with matchup spread
ex=lb[len(lb)//2]
g=heavy[ex]
print(f"\n=== EXAMPLE PLAYER: {ex}  (score {pmean[ex]:+.2f}, {pct(pmean[ex]):.0f}th pct, {len(g)} games) ===")
bymu=defaultdict(list)
for x in g: bymu[x["matchup"]].append(x["good"])
for mu in sorted(bymu): print(f"  {mu}: n={len(bymu[mu])}  score={st.mean(bymu[mu]):+.2f}")
# worst early gaps across their games (start<7min), with clock
worst=[]
for x in g:
    for t,d in x["gaps"]:
        if t<420: worst.append((d,t,x["matchup"],x["map"]))
worst.sort(reverse=True)
print("  worst early supply gaps:")
for d,t,mu,mp in worst[:6]:
    print(f"    {d:3d}s gap starting {t//60}:{t%60:02d}  ({mu}, {mp[:22]})")

# corpus distribution of per-game good (for histogram)
allgood=sorted(r["good"] for r in rows)
bins=[-3+0.5*i for i in range(13)]
hist=[sum(1 for v in allgood if bins[i]<=v<bins[i+1]) for i in range(len(bins)-1)]
print("\n=== per-game score distribution (z units) ===")
print("bins",[round(b,1) for b in bins]); print("hist",hist)
print("median",round(st.median(allgood),2),"n",len(allgood))
