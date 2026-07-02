#!/usr/bin/env python3
"""Throughput-latency + headline plots for the v2 payload-size sweep.

v2 = after fix #1 (AppendBinary: write response/request directly into shm
slot body) + fix #2 (UnmarshalBinary aliases the input slice for variable
data instead of make+copy).

Reads results/chain_payload_sweep_v2/summary.csv.
"""
import os
import pandas as pd
import matplotlib.pyplot as plt

CSV    = "results/chain_payload_sweep_v2/summary.csv"
OUTDIR = "results/chain_payload_sweep_v2"
PANELS = os.path.join(OUTDIR, "panels")
os.makedirs(PANELS, exist_ok=True)

# (payload bytes, label, matching RpcMsgSize)
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
def peak_per_backend():
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
                    concurrency=sub.loc[idx, "concurrency"],
                ))
    return pd.DataFrame(rows)


peaks = peak_per_backend()

fig, ax = plt.subplots(figsize=(8.5, 5.5))
for backend in ["http", "tcs", "cq", "cq0"]:
    sub = peaks[peaks.backend == backend].sort_values("payload")
    ax.plot(sub.payload, sub.peak_rps,
            markersize=10, markeredgecolor="black", markeredgewidth=0.5,
            **STYLE[backend])
ax.set_xscale("log", base=2)
ax.set_xlabel("Payload size (bytes, log₂)")
ax.set_ylabel("Peak throughput (rps)")
ax.set_title("chain v2 — peak throughput vs payload size\n(after fix #1 AppendBinary + fix #2 UnmarshalBinary alias)")
ax.grid(True, alpha=0.4, linestyle="--", which="both")
ax.set_ylim(bottom=0)
ax.legend(fontsize=10, framealpha=0.92)
plt.tight_layout()
out = os.path.join(OUTDIR, "peak_rps_vs_payload.png")
plt.savefig(out, dpi=140)
plt.close(fig)
print("wrote", out)

fig, ax = plt.subplots(figsize=(8.5, 5.5))
for backend in ["http", "tcs", "cq", "cq0"]:
    sub = peaks[peaks.backend == backend].sort_values("payload")
    ax.plot(sub.payload, sub.p99,
            markersize=10, markeredgecolor="black", markeredgewidth=0.5,
            **STYLE[backend])
ax.set_xscale("log", base=2)
ax.set_xlabel("Payload size (bytes, log₂)")
ax.set_ylabel("p99 latency at peak rps (ms)")
ax.set_title("chain v2 — p99 latency at peak throughput vs payload size")
ax.grid(True, alpha=0.4, linestyle="--", which="both")
ax.set_ylim(bottom=0)
ax.legend(fontsize=10, framealpha=0.92)
plt.tight_layout()
out = os.path.join(OUTDIR, "peak_p99_vs_payload.png")
plt.savefig(out, dpi=140)
plt.close(fig)
print("wrote", out)

print()
print("Peak rps per payload (v2):")
for payload, payload_label, _ in SIZES:
    print(f"  {payload_label}:")
    for backend in ["http", "tcs", "cq", "cq0"]:
        sub = df[(df.backend == backend) & (df.payload == payload)]
        if len(sub):
            idx = sub.rps.idxmax()
            r = sub.loc[idx]
            print(f"    {backend:>4}: peak {r.rps:>9,.0f} rps @ c={r.concurrency:>3}, "
                  f"p50={r.p50_ms:>5.2f} ms, p99={r.p99_ms:>6.2f} ms")


# ── v1 vs v2 comparison ────────────────────────────────────────────────────
V1_CSV = "results/chain_payload_sweep/summary.csv"
if os.path.exists(V1_CSV):
    df_v1 = pd.read_csv(V1_CSV)
    df_v1 = df_v1[df_v1["succ"] == "100.00%"].copy()

    def peak_per_backend_for(d):
        rows = []
        for backend in ["http", "tcs", "cq", "cq0"]:
            for payload, payload_label, _ in SIZES:
                sub = d[(d.backend == backend) & (d.payload == payload)]
                if len(sub):
                    idx = sub.rps.idxmax()
                    rows.append(dict(backend=backend, payload=payload,
                                     peak_rps=sub.loc[idx, "rps"],
                                     p99=sub.loc[idx, "p99_ms"]))
        return pd.DataFrame(rows)

    p1 = peak_per_backend_for(df_v1)
    p2 = peak_per_backend_for(df)

    # peak-rps comparison: solid v2, dashed v1
    fig, ax = plt.subplots(figsize=(9.0, 5.8))
    for backend in ["http", "tcs", "cq", "cq0"]:
        s1 = p1[p1.backend == backend].sort_values("payload")
        s2 = p2[p2.backend == backend].sort_values("payload")
        style = STYLE[backend]
        if len(s1):
            ax.plot(s1.payload, s1.peak_rps, marker="x", linestyle=":",
                    color=style["color"], linewidth=1.3,
                    label=f"{backend} v1")
        if len(s2):
            ax.plot(s2.payload, s2.peak_rps, marker=style["marker"], linestyle="-",
                    color=style["color"], linewidth=2.0, markersize=9,
                    markeredgecolor="black", markeredgewidth=0.5,
                    label=f"{backend} v2")
    ax.set_xscale("log", base=2)
    ax.set_xlabel("Payload size (bytes, log₂)")
    ax.set_ylabel("Peak throughput (rps)")
    ax.set_title("chain — peak throughput vs payload: v1 (before) vs v2 (fix #1+#2)")
    ax.grid(True, alpha=0.4, linestyle="--", which="both")
    ax.set_ylim(bottom=0)
    ax.legend(fontsize=8, framealpha=0.92, ncol=2)
    plt.tight_layout()
    out = os.path.join(OUTDIR, "peak_rps_v1_vs_v2.png")
    plt.savefig(out, dpi=140)
    plt.close(fig)
    print("wrote", out)

    print()
    print("v1 → v2 peak rps:")
    for payload, payload_label, _ in SIZES:
        print(f"  {payload_label}:")
        for backend in ["http", "tcs", "cq", "cq0"]:
            v1 = p1[(p1.backend == backend) & (p1.payload == payload)]
            v2 = p2[(p2.backend == backend) & (p2.payload == payload)]
            if len(v1) and len(v2):
                r1 = v1.peak_rps.iloc[0]
                r2 = v2.peak_rps.iloc[0]
                gain = (r2 - r1) / r1 * 100
                print(f"    {backend:>4}: {r1:>9,.0f} → {r2:>9,.0f} rps  ({gain:+5.1f}%)")
