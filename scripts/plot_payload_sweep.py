#!/usr/bin/env python3
"""Throughput-latency + headline plots for the payload-size sweep.

Reads results/chain_payload_sweep/summary.csv. Produces:

  panels/p50_{64B,256B,1KB,4KB,16KB,64KB}.png   per-payload p50 panels
  panels/p99_{64B,256B,1KB,4KB,16KB,64KB}.png   per-payload p99 panels
  p50_all.png                                   1xN combined p50
  p99_all.png                                   1xN combined p99
  peak_rps_vs_payload.png                       headline: peak rps vs payload
  peak_p99_vs_payload.png                       headline: p99-at-peak vs payload

Each axis on every panel starts at (0, 0). Grid is on, both axes
labeled. Each panel title includes the matching RpcMsgSize so the
"snug-fit per payload" methodology is documented in the figure.
"""
import os
import pandas as pd
import matplotlib.pyplot as plt

CSV    = "results/chain_payload_sweep/summary.csv"
OUTDIR = "results/chain_payload_sweep"
PANELS = os.path.join(OUTDIR, "panels")
os.makedirs(PANELS, exist_ok=True)

# (payload bytes, label, matching RpcMsgSize)
# 64KB payload was attempted but RpcMsgSize=128 KB doesn't work in any
# backend (single requests time out at 30s) — likely a shm-pool exhaustion
# / page-fault-storm issue at that scale. Dropped from the sweep.
SIZES = [
    (   64, "64B",   "256 B"),
    (  256, "256B",  "512 B"),
    ( 1024, "1KB",   "2 KB"),
    ( 4096, "4KB",   "8 KB"),
    (16384, "16KB",  "32 KB"),
]

STYLE = {
    "http": dict(label="http", color="tab:blue",   marker="o", linestyle="-",  linewidth=2.0),
    "tcs":  dict(label="tcs",  color="tab:orange", marker="s", linestyle="--", linewidth=1.8),
    "cq":   dict(label="cq",   color="khaki",      marker="s", linestyle="--", linewidth=1.8),
    "cq0":  dict(label="cq0",  color="tab:green",  marker="s", linestyle="--", linewidth=1.8),
}

df = pd.read_csv(CSV)
df = df[df["succ"] == "100.00%"].copy()


def draw_panel(ax, metric, payload, payload_label, msg_label):
    for backend in ["http", "tcs", "cq", "cq0"]:
        sub = df[(df.backend == backend) & (df.payload == payload)].sort_values("concurrency")
        if len(sub):
            ax.plot(sub.rps, sub[metric],
                    markersize=8, markeredgecolor="black", markeredgewidth=0.5,
                    **STYLE[backend])
    metric_short = "p50" if metric == "p50_ms" else "p99"
    ax.set_title(f"payload={payload_label}  (RpcMsgSize={msg_label})  —  {metric_short} latency",
                 fontsize=10)
    ax.set_xlabel("Throughput (requests / s)")
    ax.set_ylabel(f"{metric_short} latency (ms)")
    ax.grid(True, alpha=0.4, linestyle="--")
    ax.set_xlim(left=0)
    ax.set_ylim(bottom=0)
    sub_all = df[df.payload == payload]
    if len(sub_all):
        ax.set_xlim(0, sub_all.rps.max() * 1.05)
        ax.set_ylim(0, sub_all[metric].max() * 1.15)
    ax.legend(loc="upper left", fontsize=9, framealpha=0.92)


def save_individual(metric, metric_label):
    for payload, payload_label, msg_label in SIZES:
        fig, ax = plt.subplots(figsize=(6.5, 5.0))
        draw_panel(ax, metric, payload, payload_label, msg_label)
        plt.tight_layout()
        out = os.path.join(PANELS, f"{metric_label}_{payload_label}.png")
        plt.savefig(out, dpi=140)
        plt.close(fig)
        print("wrote", out)


def save_combined(metric, metric_label):
    n = len(SIZES)
    fig, axes = plt.subplots(1, n, figsize=(4.6 * n, 4.8))
    for ax, (payload, payload_label, msg_label) in zip(axes, SIZES):
        draw_panel(ax, metric, payload, payload_label, msg_label)
    plt.tight_layout()
    out = os.path.join(OUTDIR, f"{metric_label}_all.png")
    plt.savefig(out, dpi=140)
    plt.close(fig)
    print("wrote", out)


for metric, label in [("p50_ms", "p50"), ("p99_ms", "p99")]:
    save_individual(metric, label)
    save_combined(metric, label)


# ── Headline: peak rps and p99-at-peak vs payload size ─────────────────────
def peak_per_backend(metric):
    rows = []
    for backend in ["http", "tcs", "cq", "cq0"]:
        for payload, payload_label, _ in SIZES:
            sub = df[(df.backend == backend) & (df.payload == payload)]
            if len(sub):
                idx = sub.rps.idxmax()
                rows.append(dict(
                    backend=backend, payload=payload,
                    peak_rps=sub.loc[idx, "rps"],
                    p50=sub.loc[idx, "p50_ms"],
                    p99=sub.loc[idx, "p99_ms"],
                ))
    return pd.DataFrame(rows)


peaks = peak_per_backend("rps")

# Peak-rps plot
fig, ax = plt.subplots(figsize=(8.5, 5.5))
for backend in ["http", "tcs", "cq", "cq0"]:
    sub = peaks[peaks.backend == backend].sort_values("payload")
    ax.plot(sub.payload, sub.peak_rps,
            markersize=10, markeredgecolor="black", markeredgewidth=0.5,
            **STYLE[backend])
ax.set_xscale("log", base=2)
ax.set_xlabel("Payload size (bytes, log₂)")
ax.set_ylabel("Peak throughput (rps)")
ax.set_title("chain — peak throughput vs payload size\n(RpcMsgSize snug-fit to payload per row)")
ax.grid(True, alpha=0.4, linestyle="--", which="both")
ax.set_ylim(bottom=0)
ax.legend(fontsize=10, framealpha=0.92)
plt.tight_layout()
out = os.path.join(OUTDIR, "peak_rps_vs_payload.png")
plt.savefig(out, dpi=140)
plt.close(fig)
print("wrote", out)

# p99-at-peak plot
fig, ax = plt.subplots(figsize=(8.5, 5.5))
for backend in ["http", "tcs", "cq", "cq0"]:
    sub = peaks[peaks.backend == backend].sort_values("payload")
    ax.plot(sub.payload, sub.p99,
            markersize=10, markeredgecolor="black", markeredgewidth=0.5,
            **STYLE[backend])
ax.set_xscale("log", base=2)
ax.set_xlabel("Payload size (bytes, log₂)")
ax.set_ylabel("p99 latency at peak rps (ms)")
ax.set_title("chain — p99 latency at peak throughput vs payload size")
ax.grid(True, alpha=0.4, linestyle="--", which="both")
ax.set_ylim(bottom=0)
ax.legend(fontsize=10, framealpha=0.92)
plt.tight_layout()
out = os.path.join(OUTDIR, "peak_p99_vs_payload.png")
plt.savefig(out, dpi=140)
plt.close(fig)
print("wrote", out)

print()
print("Peak rps per payload (max-over-concurrency):")
for payload, payload_label, _ in SIZES:
    print(f"  {payload_label}:")
    for backend in ["http", "tcs", "cq", "cq0"]:
        sub = df[(df.backend == backend) & (df.payload == payload)]
        if len(sub):
            idx = sub.rps.idxmax()
            r = sub.loc[idx]
            print(f"    {backend:>4}: peak {r.rps:>9,.0f} rps @ c={r.concurrency:>3}, "
                  f"p50={r.p50_ms:>5.2f} ms, p99={r.p99_ms:>6.2f} ms")
