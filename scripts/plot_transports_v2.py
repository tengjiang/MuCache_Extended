#!/usr/bin/env python3
"""Throughput-latency plots for chain transports — v2 with finer sweep.

Produces 10 PNGs from results/chain_transports_v2/summary.csv:

  panels/p50_{512B,2KB,8KB,32KB}.png   — one per msg-size, p50 latency
  panels/p99_{512B,2KB,8KB,32KB}.png   — one per msg-size, p99 latency
  p50_all.png                          — 1x4 combined p50
  p99_all.png                          — 1x4 combined p99

Each axis starts at (0, 0). Grid is on. Both axes are labeled. All
sub-panels are saved individually AND combined into the all-panels
figure. http is plotted on every msg-size panel (it has no shm
frame, so the same row is reused).
"""
import os
import pandas as pd
import matplotlib.pyplot as plt

CSV     = "results/chain_transports_v2/summary.csv"
OUTDIR  = "results/chain_transports_v2"
PANELS  = os.path.join(OUTDIR, "panels")
os.makedirs(PANELS, exist_ok=True)

SIZES = [(512, "512B"), (2048, "2KB"), (8192, "8KB"), (32768, "32KB")]

STYLE = {
    "http": dict(label="http", color="tab:blue",   marker="o", linestyle="-",  linewidth=2.0),
    "tcs":  dict(label="tcs",  color="tab:orange", marker="s", linestyle="--", linewidth=1.8),
    "cq":   dict(label="cq",   color="khaki",      marker="s", linestyle="--", linewidth=1.8),
    "cq0":  dict(label="cq0",  color="tab:green",  marker="s", linestyle="--", linewidth=1.8),
}

df = pd.read_csv(CSV)
df = df[df["succ"] == "100.00%"].copy()
http = df[df.backend == "http"].sort_values("concurrency")


def draw_panel(ax, metric, size_bytes, size_label):
    """Render one panel onto a given axes object."""
    # http baseline (no msg_size — reuse on every panel)
    ax.plot(http.rps, http[metric],
            markersize=8, markeredgecolor="black", markeredgewidth=0.5,
            **STYLE["http"])
    # three flame backends
    for backend in ["tcs", "cq", "cq0"]:
        sub = df[(df.backend == backend) & (df.msg_size == float(size_bytes))].sort_values("concurrency")
        ax.plot(sub.rps, sub[metric],
                markersize=8, markeredgecolor="black", markeredgewidth=0.5,
                **STYLE[backend])

    metric_label = "p50" if metric == "p50_ms" else "p99"
    ax.set_title(f"chain (RpcMsgSize = {size_label})  —  {metric_label} latency", fontsize=11)
    ax.set_xlabel("Throughput (requests / s)")
    ax.set_ylabel(f"{metric_label} latency (ms)")
    ax.grid(True, alpha=0.4, linestyle="--")
    ax.set_xlim(left=0)
    ax.set_ylim(bottom=0)
    # Cap y at the data max + headroom; cap x at 65k for consistency
    ymax = max(http[metric].max(), df[df.msg_size == float(size_bytes)][metric].max()) * 1.15
    xmax = max(http.rps.max(), df[df.msg_size == float(size_bytes)].rps.max()) * 1.05
    ax.set_xlim(0, max(xmax, 1))
    ax.set_ylim(0, max(ymax, 1))
    ax.legend(loc="upper left", fontsize=10, framealpha=0.92)


def save_individual(metric, metric_label):
    """Save 4 individual panels (one per msg-size)."""
    for size_bytes, size_label in SIZES:
        fig, ax = plt.subplots(figsize=(6.5, 5.0))
        draw_panel(ax, metric, size_bytes, size_label)
        plt.tight_layout()
        out = os.path.join(PANELS, f"{metric_label}_{size_label}.png")
        plt.savefig(out, dpi=140)
        plt.close(fig)
        print("wrote", out)


def save_combined(metric, metric_label):
    """Save the 1x4 combined figure."""
    fig, axes = plt.subplots(1, 4, figsize=(22, 5.0))
    for ax, (size_bytes, size_label) in zip(axes, SIZES):
        draw_panel(ax, metric, size_bytes, size_label)
    plt.tight_layout()
    out = os.path.join(OUTDIR, f"{metric_label}_all.png")
    plt.savefig(out, dpi=140)
    plt.close(fig)
    print("wrote", out)


for metric, label in [("p50_ms", "p50"), ("p99_ms", "p99")]:
    save_individual(metric, label)
    save_combined(metric, label)

# Print headline numbers
print()
print("Peak rps (c=128):")
for size_bytes, size_label in SIZES:
    print(f"  {size_label}:")
    for backend in ["http", "tcs", "cq", "cq0"]:
        if backend == "http":
            row = http[http.concurrency == 128]
        else:
            row = df[(df.backend == backend) & (df.msg_size == float(size_bytes)) & (df.concurrency == 128)]
        if len(row):
            r = row.iloc[0]
            print(f"    {backend:>4}: {r.rps:>9,.0f} rps  p50={r.p50_ms:.2f}ms  p99={r.p99_ms:.2f}ms")
