#!/usr/bin/env python3
"""Throughput-latency curves for 4 chain transport backends at one msg-size.

Mirrors the reference paper plot: 4 lines (http baseline + TCS, CQ, CQ0
shared-memory variants), throughput on x and latency on y.

Input:  results/chain_transports/summary.csv
Output: results/chain_transports/chain_transports.png
"""
import pandas as pd
import matplotlib.pyplot as plt

CSV = "results/chain_transports/summary.csv"
OUT = "results/chain_transports/chain_transports.png"

df = pd.read_csv(CSV)
df = df[df["succ"] == "100.00%"]

# Match the visual style of the reference plot.
STYLE = {
    "http": dict(label="http",  color="tab:blue",      marker="o", linestyle="-",  linewidth=2.0),
    "tcs":  dict(label="tcs",   color="tab:orange",    marker="s", linestyle="--", linewidth=1.8),
    "cq":   dict(label="cq",    color="khaki",         marker="s", linestyle="--", linewidth=1.8),
    "cq0":  dict(label="cq0",   color="tab:green",     marker="s", linestyle="--", linewidth=1.8),
}

fig, axes = plt.subplots(1, 2, figsize=(13, 5.2))

for ax, metric, ylabel in [(axes[0], "p50_ms", "p50 latency (ms)"),
                           (axes[1], "p99_ms", "p99 latency (ms)")]:
    for backend, style in STYLE.items():
        sub = df[df.backend == backend].sort_values("concurrency")
        ax.plot(sub.rps, sub[metric], markersize=9, markeredgecolor="black",
                markeredgewidth=0.5, **style)
    ax.set_xlabel("Throughput (requests / s)")
    ax.set_ylabel(ylabel)
    ax.set_title(f"chain (local N0) — {ylabel.split(' (')[0]} vs throughput")
    ax.grid(True, alpha=0.3)
    ax.legend(loc="upper left", fontsize=10, framealpha=0.92)
    ax.set_ylim(bottom=0)
    ax.set_xlim(left=0)

plt.tight_layout()
plt.savefig(OUT, dpi=140)
print("wrote", OUT)
print()
print("Peak rps (c=128):")
for b in ["http", "tcs", "cq", "cq0"]:
    row = df[(df.backend == b) & (df.concurrency == 128)]
    if len(row):
        print(f"  {b:>4}: {row.rps.values[0]:>9,.0f} rps  p50={row.p50_ms.values[0]:.2f}ms  p99={row.p99_ms.values[0]:.2f}ms")
