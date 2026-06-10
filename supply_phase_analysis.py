import sqlite3, statistics as st, sys, math
from collections import defaultdict

DB = sys.argv[1] if len(sys.argv) > 1 else "cwal.db"
PROV = {("Build","Supply Depot"):8,("Build","Pylon"):8,("Build","Hatchery"):1,
        ("Build","Command Center"):10,("Build","Nexus"):9,("Unit Morph","Overlord"):8}
SEED = {"Terran":10,"Protoss":9,"Zerg":9}
SUPPLY_CAP=200; MIN_ADDS=4
EARLY_END=420; MID_END=900   # 7 min, 15 min
MIN_GAMES=10

con=sqlite3.connect(DB); con.row_factory=sqlite3.Row
players={r["id"]:dict(r) for r in con.execute(
  "SELECT p.id,p.name,p.race,p.is_winner,r.duration_seconds dur,r.matchup,p.replay_id rid "
  "FROM players p JOIN replays r ON r.id=p.replay_id WHERE p.type='Human'")}
events=defaultdict(list)
for r in con.execute("SELECT player_id,action_type,unit_type,seconds_from_game_start sec FROM commands"):
    v=PROV.get((r["action_type"],r["unit_type"]))
    if v: events[r["player_id"]].append((r["sec"],v))

# weighting schemes over a gap's START second
SCHEMES={
 "flat":      lambda t: 1.0,
 "step":      lambda t: 1.0 if t<EARLY_END else (0.5 if t<MID_END else 0.1),  # user's intuition
 "linear15":  lambda t: max(0.0, 1.0 - t/MID_END),                            # ->0 at 15min
 "exp5":      lambda t: math.exp(-t/300.0),                                   # half-life ~3.5min
 "exp8":      lambda t: math.exp(-t/480.0),
}

def gaps_of(pid,p):
    evs=sorted(events.get(pid,[]))
    seq=sorted([(0,SEED.get(p["race"],0))]+[(max(0,s),v) for s,v in evs])
    end=int(0.8*p["dur"]); times,cum=[],0
    for s,v in seq:
        cum+=v
        if s>end: break
        times.append(s)
        if cum>=SUPPLY_CAP: break
    if len(times)<MIN_ADDS: return None
    if times[-1]-times[0]<120: return None
    return [(times[i],times[i+1]-times[i]) for i in range(len(times)-1)]  # (start,dur)

rows=[]
for pid,p in players.items():
    g=gaps_of(pid,p)
    if not g: continue
    rec=dict(name=p["name"],matchup=p["matchup"],is_winner=p["is_winner"],rid=p["rid"],gaps=g)
    # weighted means
    for s,fn in SCHEMES.items():
        ws=[fn(t) for t,_ in g]; sw=sum(ws)
        rec["w_"+s]=sum(w*d for w,(_,d) in zip(ws,g))/sw if sw else None
    # phase means (avg gap duration for gaps starting in phase)
    for ph,lo,hi in [("early",0,EARLY_END),("mid",EARLY_END,MID_END),("late",MID_END,10**9)]:
        ds=[d for t,d in g if lo<=t<hi]
        rec["ph_"+ph]=st.mean(ds) if len(ds)>=2 else None
    rows.append(rec)
print(f"DB={DB}  games={len(rows)}\n")

def znorm(rows, key):
    by=defaultdict(list)
    for r in rows:
        if r.get(key) is not None: by[r["matchup"]].append(r[key])
    stt={k:(st.mean(v),st.pstdev(v) or 1) for k,v in by.items()}
    out={}
    for r in rows:
        if r.get(key) is None: continue
        m,sd=stt[r["matchup"]]; out[r["rid"],r["name"]]=(r[key]-m)/sd
    return out

def winloss(rows,key):  # mu-normalized winner-loser diff in SD
    z=znorm(rows,key)
    w=[v for r in rows if (k:=(r["rid"],r["name"])) in z and r["is_winner"] for v in [z[k]]]
    l=[v for r in rows if (k:=(r["rid"],r["name"])) in z and not r["is_winner"] for v in [z[k]]]
    return st.mean(w)-st.mean(l), len(w)+len(l)

def pearson(a,b):
    if len(a)<3: return 0
    ma,mb=st.mean(a),st.mean(b); cov=sum((x-ma)*(y-mb) for x,y in zip(a,b))
    da=sum((x-ma)**2 for x in a)**.5; db=sum((y-mb)**2 for y in b)**.5
    return cov/(da*db) if da and db else 0

def splithalf(rows,key):
    z=znorm(rows,key); byname=defaultdict(list)
    for r in rows:
        k=(r["rid"],r["name"])
        if k in z: byname[r["name"]].append((r["rid"],z[k]))
    a,b=[],[]
    for n,lst in byname.items():
        if len(lst)<MIN_GAMES: continue
        lst.sort()
        ha=[v for i,(_,v) in enumerate(lst) if i%2==0]; hb=[v for i,(_,v) in enumerate(lst) if i%2==1]
        if ha and hb: a.append(st.mean(ha)); b.append(st.mean(hb))
    return pearson(a,b), len(a)

print("=== PHASE: where does the win-signal live? (mu-normalized winner-loser diff, SD) ===")
print("   (negative = winners have shorter gaps in that phase = good)")
for ph in ["early","mid","late"]:
    d,n=winloss(rows,"ph_"+ph); sh,np_=splithalf(rows,"ph_"+ph)
    cov=sum(1 for r in rows if r.get("ph_"+ph) is not None)
    print(f"  {ph:5s}  win-loss_diff={d:+.3f}  split_half={sh:+.2f}({np_}p)  games_with_phase={cov}")

print("\n=== WEIGHTING SCHEMES: full-game weighted mean_gap ===")
print("   bigger |win-loss diff| = better outcome signal; split_half = reliability")
for s in SCHEMES:
    d,n=winloss(rows,"w_"+s); sh,np_=splithalf(rows,"w_"+s)
    print(f"  {s:9s} win-loss_diff={d:+.3f}  split_half={sh:+.2f}")
