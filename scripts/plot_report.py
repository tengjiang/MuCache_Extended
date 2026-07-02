#!/usr/bin/env python3
"""Report figure for the flame/rpc copy-elimination commits.

Two panels:
  Left:  throughput-latency curves (p99) at 2048/8192/16384, before vs after
         the optimizations — visualizes how the after-curve shifts right
         (higher peak rps) and down (lower tail latency at the same load).
  Right: peak rps improvement (%) vs RpcMsgSize — the headline finding that
         the gain scales with frame size.

Input:  results/chain_msgsize/summary.csv
Output: results/chain_msgsize/report.png
"""
import pandas as pd
import matplotlib.pyplot as plt
import numpy as np

CSV   = "results/chain_msgsize/summary.csv"
OUT   = "results/chain_msgsize/report.png"
SIZES = [2048, 8192, 16384]
COLOR = {2048: "tab:blue", 8192: "tab:green", 16384: "tab:red"}

df = pd.read_csv(CSV)
df = df[df["success_rate"] == "100.00%"]
flame = df[df.series == "flame"].copy();     flame["msg_size"] = flame.msg_size.astype(int)
opt   = df[df.series == "flame_opt"].copy(); opt["msg_size"]   = opt.msg_size.astype(int)

fig, (axL, axR) = plt.subplots(1, 2, figsize=(13.5, 5.2), gridspec_kw={"width_ratios": [1.5, 1]})

# ── Left: p99 latency vs throughput, before/after at each size ─────────────
for s in SIZES:
    b = flame[flame.msg_size == s].sort_values("concurrency")
    o = opt  [opt.msg_size   == s].sort_values("concurrency")
    axL.plot(b.rps, b.p99_ms, marker="o", linestyle="--", color=COLOR[s], alpha=0.55,
             label=f"{s} B  before")
    axL.plot(o.rps, o.p99_ms, marker="*", markersize=11, linewidth=2.2, color=COLOR[s],
             label=f"{s} B  after")
axL.set_xlabel("Throughput (requests / s)")
axL.set_ylabel("p99 latency (ms)")
axL.set_title("chain — p99 latency vs throughput\nbefore vs after copy-elimination commits")
axL.grid(True, alpha=0.3)
axL.legend(loc="upper left", fontsize=9, framealpha=0.92)
axL.set_ylim(bottom=0)

# ── Right: % peak-rps gain by frame size ──────────────────────────────────
deltas = []
labels = []
for s in SIZES:
    b = flame[(flame.msg_size == s) & (flame.concurrency == 128)].rps.values[0]
    o = opt  [(opt.msg_size   == s) & (opt.concurrency   == 128)].rps.values[0]
    deltas.append((o - b) / b * 100)
    labels.append(f"{s} B\n({b:,.0f} → {o:,.0f})")
bars = axR.bar(range(len(SIZES)), deltas,
               color=[COLOR[s] for s in SIZES],
               edgecolor="black", linewidth=0.8)
for bar, d in zip(bars, deltas):
    axR.text(bar.get_x() + bar.get_width() / 2, bar.get_height() + 0.3,
             f"+{d:.1f}%", ha="center", va="bottom", fontsize=11, fontweight="bold")
axR.set_xticks(range(len(SIZES)))
axR.set_xticklabels(labels, fontsize=9)
axR.set_ylabel("Peak rps improvement (%)  —  c=128")
axR.set_title("Gain scales with frame size\n(removed work is msg_size-proportional)")
axR.set_ylim(0, max(deltas) * 1.25)
axR.grid(True, axis="y", alpha=0.3)

plt.tight_layout()
plt.savefig(OUT, dpi=140)
print("wrote", OUT)
print()
print("Headline numbers (peak rps, c=128):")
for s, d in zip(SIZES, deltas):
    b = flame[(flame.msg_size == s) & (flame.concurrency == 128)].rps.values[0]
    o = opt  [(opt.msg_size   == s) & (opt.concurrency   == 128)].rps.values[0]
    print(f"  {s:>5} B :  {b:>8,.0f}  →  {o:>8,.0f}   (+{d:5.1f}%)")
