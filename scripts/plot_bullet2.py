#!/usr/bin/env python3
"""Bullet 2 (slot-pointer / zero-copy shm) on top of Phase 1 (kill JSON).

Two panels:
  Left:  p99 latency vs throughput curves at 3 sizes, original → Phase 0
         → Phase 1 → Bullet 2 (slot).
  Right: stacked-bar cumulative gain by phase (peak rps, c=128).

Input:  results/chain_msgsize/summary.csv
Output: results/chain_msgsize/bullet2_report.png
"""
import pandas as pd
import matplotlib.pyplot as plt
import numpy as np

CSV   = "results/chain_msgsize/summary.csv"
OUT   = "results/chain_msgsize/bullet2_report.png"
SIZES = [2048, 8192, 16384]
COLOR = {2048: "tab:blue", 8192: "tab:green", 16384: "tab:red"}

df = pd.read_csv(CSV)
df = df[df["success_rate"] == "100.00%"]
def series(n):
    s = df[df.series == n].copy(); s["msg_size"] = s.msg_size.astype(int); return s
flame  = series("flame")        # original
opt    = series("flame_opt")    # Phase 0
binary = series("flame_binary") # Phase 1
slot   = series("flame_slot")   # Bullet 2

fig, (axL, axR) = plt.subplots(1, 2, figsize=(14, 5.5), gridspec_kw={"width_ratios": [1.6, 1]})

# ── Left: layered curves ─────────────────────────────────────────────────
for s in SIZES:
    a = flame [flame .msg_size == s].sort_values("concurrency")
    b = opt   [opt   .msg_size == s].sort_values("concurrency")
    c = binary[binary.msg_size == s].sort_values("concurrency")
    d = slot  [slot  .msg_size == s].sort_values("concurrency")
    axL.plot(a.rps, a.p99_ms, marker="o", linestyle=":",   color=COLOR[s], alpha=0.40, linewidth=1.2, label=f"{s}B  original")
    axL.plot(b.rps, b.p99_ms, marker="s", linestyle="--",  color=COLOR[s], alpha=0.55, linewidth=1.6, label=f"{s}B  +Phase 0")
    axL.plot(c.rps, c.p99_ms, marker="*", linestyle="-.",  color=COLOR[s], alpha=0.75, linewidth=1.8, markersize=8,  label=f"{s}B  +Phase 1")
    axL.plot(d.rps, d.p99_ms, marker="D", linestyle="-",   color=COLOR[s], linewidth=2.6, markersize=8,
             markeredgecolor="black", markeredgewidth=0.6, label=f"{s}B  +Bullet 2 (slot)")
axL.set_xlabel("Throughput (requests / s)")
axL.set_ylabel("p99 latency (ms)")
axL.set_title("chain — p99 vs throughput\nlayered across Phase 0 (rpc.go) + Phase 1 (kill JSON) + Bullet 2 (zero-copy shm)")
axL.grid(True, alpha=0.3)
axL.legend(loc="upper left", fontsize=7, framealpha=0.92, ncol=1)
axL.set_ylim(bottom=0)

# ── Right: stacked-bar contribution per phase (peak rps c=128) ──────────
peak = lambda d, s: d[(d.msg_size == s) & (d.concurrency == 128)].rps.values[0]
base = [peak(flame , s) for s in SIZES]
ph0  = [peak(opt   , s) - peak(flame , s) for s in SIZES]
ph1  = [peak(binary, s) - peak(opt   , s) for s in SIZES]
ph2  = [peak(slot  , s) - peak(binary, s) for s in SIZES]

x = np.arange(len(SIZES)); w = 0.65
axR.bar(x, base, w, color=[COLOR[s] for s in SIZES], alpha=0.35, edgecolor="black", linewidth=0.8, label="original")
axR.bar(x, ph0,  w, bottom=base,                                            color=[COLOR[s] for s in SIZES], alpha=0.55, edgecolor="black", linewidth=0.8, hatch="//", label="Phase 0")
axR.bar(x, ph1,  w, bottom=[a+b for a,b in zip(base,ph0)],                  color=[COLOR[s] for s in SIZES], alpha=0.75, edgecolor="black", linewidth=0.8, hatch="xx", label="Phase 1")
axR.bar(x, ph2,  w, bottom=[a+b+c for a,b,c in zip(base,ph0,ph1)],          color=[COLOR[s] for s in SIZES], alpha=1.00, edgecolor="black", linewidth=0.8, hatch="..", label="Bullet 2")
for i, s in enumerate(SIZES):
    total = peak(slot, s)
    cum = (total - peak(flame, s)) / peak(flame, s) * 100
    axR.text(i, total + 800, f"+{cum:.0f}%\n{total:,.0f} rps",
             ha="center", va="bottom", fontsize=10, fontweight="bold")
axR.set_xticks(x); axR.set_xticklabels([f"{s} B" for s in SIZES])
axR.set_ylabel("Peak rps (c=128)")
axR.set_title("Cumulative gain by phase\n(label = +% vs original baseline)")
axR.set_ylim(0, max(peak(slot, s) for s in SIZES) * 1.20)
axR.legend(loc="upper left", fontsize=9, framealpha=0.92)
axR.grid(True, axis="y", alpha=0.3)

plt.tight_layout()
plt.savefig(OUT, dpi=140)
print("wrote", OUT)
print()
print("Headline (peak rps, c=128):")
print(f"  {'size':>6} | {'orig':>9} | {'Ph 0':>9} | {'Ph 1':>9} | {'Bullet 2':>9} | {'cum Δ':>7}")
for s in SIZES:
    a, b, c, d = peak(flame, s), peak(opt, s), peak(binary, s), peak(slot, s)
    print(f"  {s:>5}B | {a:>9,.0f} | {b:>9,.0f} | {c:>9,.0f} | {d:>9,.0f} | {(d-a)/a*100:>+6.1f}%")
