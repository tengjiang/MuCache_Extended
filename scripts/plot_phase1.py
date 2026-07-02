#!/usr/bin/env python3
"""Phase 1 (kill JSON for chain) on top of Phase 0 (rpc.go opts).

Two panels:
  Left:  p99 latency vs throughput at three frame sizes, three series each
         (original → +Phase 0 → +Phase 1).
  Right: cumulative peak-rps gain by frame size, broken into the two phases.

Input:  results/chain_msgsize/summary.csv
Output: results/chain_msgsize/phase1_report.png
"""
import pandas as pd
import matplotlib.pyplot as plt
import numpy as np

CSV   = "results/chain_msgsize/summary.csv"
OUT   = "results/chain_msgsize/phase1_report.png"
SIZES = [2048, 8192, 16384]
COLOR = {2048: "tab:blue", 8192: "tab:green", 16384: "tab:red"}

df = pd.read_csv(CSV)
df = df[df["success_rate"] == "100.00%"]
def series(name):
    s = df[df.series == name].copy(); s["msg_size"] = s.msg_size.astype(int); return s
flame  = series("flame")          # original (pre-Phase-0)
opt    = series("flame_opt")      # Phase 0 (rpc.go optimizations)
binary = series("flame_binary")   # Phase 1 (binary encoding)

fig, (axL, axR) = plt.subplots(1, 2, figsize=(14, 5.4), gridspec_kw={"width_ratios": [1.6, 1]})

# ── Left: layered throughput-latency curves ────────────────────────────────
for s in SIZES:
    a = flame [flame .msg_size == s].sort_values("concurrency")
    b = opt   [opt   .msg_size == s].sort_values("concurrency")
    c = binary[binary.msg_size == s].sort_values("concurrency")
    axL.plot(a.rps, a.p99_ms, marker="o", linestyle=":",   color=COLOR[s], alpha=0.45,
             linewidth=1.4, label=f"{s} B  original")
    axL.plot(b.rps, b.p99_ms, marker="s", linestyle="--",  color=COLOR[s], alpha=0.80,
             linewidth=1.8, label=f"{s} B  +Phase 0")
    axL.plot(c.rps, c.p99_ms, marker="*", linestyle="-",   color=COLOR[s],
             linewidth=2.4, markersize=11, label=f"{s} B  +Phase 1 (binary)")
axL.set_xlabel("Throughput (requests / s)")
axL.set_ylabel("p99 latency (ms)")
axL.set_title("chain — p99 vs throughput\nlayered across Phase 0 (rpc.go) + Phase 1 (kill JSON)")
axL.grid(True, alpha=0.3)
axL.legend(loc="upper left", fontsize=8, framealpha=0.92, ncol=1)
axL.set_ylim(bottom=0)

# ── Right: stacked-bar of contribution per phase, peak rps (c=128) ────────
peak = lambda d, s: d[(d.msg_size == s) & (d.concurrency == 128)].rps.values[0]
base  = [peak(flame , s) for s in SIZES]
ph0   = [peak(opt   , s) - peak(flame, s) for s in SIZES]
ph1   = [peak(binary, s) - peak(opt  , s) for s in SIZES]

x = np.arange(len(SIZES))
w = 0.65
axR.bar(x, base, w, color=[COLOR[s] for s in SIZES], alpha=0.45, edgecolor="black",
        linewidth=0.8, label="original")
axR.bar(x, ph0,  w, bottom=base, color=[COLOR[s] for s in SIZES], alpha=0.78,
        edgecolor="black", linewidth=0.8, hatch="//", label="Phase 0 contribution")
axR.bar(x, ph1,  w, bottom=[a + b for a, b in zip(base, ph0)],
        color=[COLOR[s] for s in SIZES], alpha=1.0, edgecolor="black",
        linewidth=0.8, hatch="xx", label="Phase 1 contribution")
for i, s in enumerate(SIZES):
    total = peak(binary, s)
    cum   = (total - peak(flame, s)) / peak(flame, s) * 100
    axR.text(i, total + 600, f"+{cum:.1f}%\n{total:,.0f} rps",
             ha="center", va="bottom", fontsize=10, fontweight="bold")
axR.set_xticks(x)
axR.set_xticklabels([f"{s} B" for s in SIZES])
axR.set_ylabel("Peak rps (c=128)")
axR.set_title("Cumulative gain by phase\n(label = +% vs original baseline)")
axR.set_ylim(0, max(base) * 1.20 + max(ph0) + max(ph1))
axR.legend(loc="upper right", fontsize=9, framealpha=0.92)
axR.grid(True, axis="y", alpha=0.3)

plt.tight_layout()
plt.savefig(OUT, dpi=140)
print("wrote", OUT)

print()
print("Headline (peak rps, c=128):")
print(f"  {'size':>6} | {'orig':>9} | {'Phase 0':>9} | {'Phase 1':>9} | {'cum Δ':>7}")
for s in SIZES:
    a, b, c = peak(flame, s), peak(opt, s), peak(binary, s)
    print(f"  {s:>5}B | {a:>9,.0f} | {b:>9,.0f} | {c:>9,.0f} | {(c-a)/a*100:>+6.1f}%")
