#!/usr/bin/env python3
"""3-panel throughput-latency plot mirroring the flame paper reference.

Each panel is one RpcMsgSize (512 / 2048 / 8192 B). Each panel has 4
lines: http baseline + tcs + cq + cq0. Reads
results/chain_transports_full/summary.csv (backend, msg_size,
concurrency, p50_ms, p99_ms, rps, succ). The http row's msg_size is
"N/A" — we reuse it on all three panels because http has no shm frame.
"""
import pandas as pd
import matplotlib.pyplot as plt

CSV   = "results/chain_transports_full/summary.csv"
OUT   = "results/chain_transports_full/chain_transports_full.png"
SIZES = [512, 2048, 8192]
METRIC = "p99_ms"   # match the reference paper's plot (latency axis)
YLABEL = "Latency (ms)"

STYLE = {
    "http": dict(label="http", color="tab:blue",   marker="o", linestyle="-",  linewidth=2.0),
    "tcs":  dict(label="tcs",  color="tab:orange", marker="s", linestyle="--", linewidth=1.8),
    "cq":   dict(label="cq",   color="khaki",      marker="s", linestyle="--", linewidth=1.8),
    "cq0":  dict(label="cq0",  color="tab:green",  marker="s", linestyle="--", linewidth=1.8),
}

df = pd.read_csv(CSV)
df = df[df["succ"] == "100.00%"]

fig, axes = plt.subplots(1, 3, figsize=(15, 4.8), sharey=True)

http = df[df.backend == "http"].sort_values("concurrency")

for ax, size in zip(axes, SIZES):
    # http baseline (size-independent)
    ax.plot(http.rps, http[METRIC],
            markersize=9, markeredgecolor="black", markeredgewidth=0.5,
            **STYLE["http"])
    # three flame backends at this size
    for backend in ["tcs", "cq", "cq0"]:
        sub = df[(df.backend == backend) & (df.msg_size == float(size))].sort_values("concurrency")
        ax.plot(sub.rps, sub[METRIC],
                markersize=9, markeredgecolor="black", markeredgewidth=0.5,
                **STYLE[backend])

    ax.set_title(f"chain (RpcMsgSize = {size} B)", fontsize=11)
    ax.set_xlabel("Throughput (requests / s)")
    ax.grid(True, alpha=0.3)
    ax.set_xlim(0, 60000)
    ax.set_ylim(0, 14)

axes[0].set_ylabel(YLABEL)
axes[0].legend(loc="upper left", fontsize=10, framealpha=0.92)
plt.tight_layout()
plt.savefig(OUT, dpi=140)
print("wrote", OUT)
print()
print("Peak rps (c=128) per size, per backend:")
for size in SIZES:
    print(f"  RpcMsgSize={size}:")
    for backend in ["http", "tcs", "cq", "cq0"]:
        if backend == "http":
            row = http[http.concurrency == 128]
        else:
            row = df[(df.backend == backend) & (df.msg_size == float(size)) & (df.concurrency == 128)]
        if len(row):
            r = row.iloc[0]
            print(f"    {backend:>4}: {r.rps:>9,.0f} rps  p50={r.p50_ms:.2f}ms  p99={r.p99_ms:.2f}ms")
