#!/usr/bin/env python3
"""Plot throughput-latency curves, one line per RpcMsgSize (+ http baseline).

Input:  results/chain_msgsize/summary.csv
Output: results/chain_msgsize/msgsize_curves.png
"""
import os, sys
import pandas as pd
import matplotlib.pyplot as plt

CSV = "results/chain_msgsize/summary.csv"
OUT = "results/chain_msgsize/msgsize_curves.png"

df = pd.read_csv(CSV)
df = df[df["success_rate"] == "100.00%"]

flame_df = df[df.series == "flame"].copy()
flame_df["msg_size"] = flame_df["msg_size"].astype(int)
flame_sizes = sorted(flame_df.msg_size.unique())
cmap = plt.cm.viridis
colors = {s: cmap(i / max(1, len(flame_sizes) - 1)) for i, s in enumerate(flame_sizes)}

fig, axes = plt.subplots(1, 2, figsize=(13, 5.2), sharex=True)
for ax, metric, label in [(axes[0], "p50_ms", "p50 latency"),
                          (axes[1], "p99_ms", "p99 latency")]:
    # http baseline
    h = df[df.series == "http"].sort_values("concurrency")
    ax.plot(h.rps, h[metric], marker="o", linewidth=2.2, color="tab:blue", label="http")
    # one line per flame msg_size
    for s in flame_sizes:
        sub = flame_df[flame_df.msg_size == s].sort_values("concurrency")
        ax.plot(sub.rps, sub[metric], marker="s", linestyle="--",
                color=colors[s], label=f"flame {s}B")
    # optimized series (pooled response buf, body view, no full-frame memset)
    opt_df = df[df.series == "flame_opt"].copy()
    if len(opt_df):
        opt_df["msg_size"] = opt_df["msg_size"].astype(int)
        opt_markers = {2048: "*", 8192: "P", 16384: "X"}
        opt_colors  = {2048: "tab:red", 8192: "tab:orange", 16384: "crimson"}
        for s in sorted(opt_df.msg_size.unique()):
            sub = opt_df[opt_df.msg_size == s].sort_values("concurrency")
            ax.plot(sub.rps, sub[metric],
                    marker=opt_markers.get(s, "*"), markersize=11, linewidth=2.2,
                    color=opt_colors.get(s, "tab:red"),
                    label=f"flame {s}B (optimized)")
    ax.set_xlabel("Throughput (requests / s)")
    ax.set_ylabel(f"{label} (ms)")
    ax.set_title(f"chain — local N0 — {label} vs throughput")
    ax.grid(True, alpha=0.3)
    ax.set_ylim(bottom=0)

axes[1].legend(loc="upper left", fontsize=8, framealpha=0.9, ncol=2)

plt.tight_layout()
os.makedirs(os.path.dirname(OUT), exist_ok=True)
plt.savefig(OUT, dpi=130)
print(f"wrote {OUT}")
