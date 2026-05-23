#!/usr/bin/env python3
"""Before/after comparison: original vs optimized flame at 3 frame sizes.

Optimizations (pkg/flame/rpc.go on distributed branch):
  1) Drop full-frame memset in rpcEncode{Request,Response}
  2) rpcDecodeBodyView (slice alias) replaces rpcDecodeBody (alloc + copy)
  3) sync.Pool for server response encode buffers
"""
import pandas as pd
import matplotlib.pyplot as plt

CSV = "results/chain_msgsize/summary.csv"
OUT = "results/chain_msgsize/optimization_compare.png"
SIZES = [2048, 8192, 16384]

df = pd.read_csv(CSV)
df = df[df["success_rate"] == "100.00%"]

flame = df[df.series == "flame"].copy(); flame["msg_size"] = flame["msg_size"].astype(int)
opt   = df[df.series == "flame_opt"].copy(); opt["msg_size"] = opt["msg_size"].astype(int)

fig, axes = plt.subplots(1, 2, figsize=(13, 5.0), sharex=False)
colors = {2048: "tab:blue", 8192: "tab:green", 16384: "tab:red"}

for ax, metric, label in [(axes[0], "p50_ms", "p50 latency"),
                          (axes[1], "p99_ms", "p99 latency")]:
    for s in SIZES:
        b = flame[flame.msg_size == s].sort_values("concurrency")
        o = opt  [opt.msg_size   == s].sort_values("concurrency")
        ax.plot(b.rps, b[metric], marker="o", linestyle="--", color=colors[s], alpha=0.55,
                label=f"{s}B before")
        ax.plot(o.rps, o[metric], marker="*", markersize=11, linewidth=2.2, color=colors[s],
                label=f"{s}B after")
    ax.set_xlabel("Throughput (requests / s)")
    ax.set_ylabel(f"{label} (ms)")
    ax.set_title(f"chain — {label} vs throughput (before vs after)")
    ax.grid(True, alpha=0.3)
    ax.set_ylim(bottom=0)

axes[0].legend(loc="upper left", fontsize=9, framealpha=0.92)
plt.tight_layout()
plt.savefig(OUT, dpi=130)
print("wrote", OUT)

# Also print the peak-rps comparison table.
print("\nPeak rps (c=128):")
for s in SIZES:
    b = flame[(flame.msg_size == s) & (flame.concurrency == 128)].rps.values
    o = opt  [(opt.msg_size   == s) & (opt.concurrency   == 128)].rps.values
    if len(b) and len(o):
        delta = (o[0] - b[0]) / b[0] * 100
        print(f"  {s:>5}B: {b[0]:>8.0f} -> {o[0]:>8.0f}  ({delta:+5.1f}%)")
