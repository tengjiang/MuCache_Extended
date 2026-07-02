#!/usr/bin/env python3
"""
Plot read/write ratio sensitivity for chain.

Reads results/chain_rw_<stamp>/r{0,20,50,80,100}/summary.csv and produces a
2-panel figure: throughput vs concurrency (target rate) and p99 latency vs
throughput, colour-coded by READ_PCT, line-style by mode (nocm / flame).

Output: results/figs/chain_rw_<stamp>.png
"""

import argparse
import os
import sys
from glob import glob

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as ticker
import pandas as pd

LAT_SCALE = 1000

# Five ratios → blues for nocm, oranges for flame, brighter = more reads.
RATIO_TO_COLOR = {
    100: "#08306b",  # all reads, darkest blue
    80:  "#2171b5",
    50:  "#6baed6",
    20:  "#bdd7e7",
    0:   "#eff3ff",  # all writes
}

NOCM_PALETTE = ["#08306b", "#2171b5", "#6baed6", "#bdd7e7", "#c6dbef"]
FLAME_PALETTE = ["#7f2704", "#d94801", "#f16913", "#fdae6b", "#fdd0a2"]


def find_root(stamp_or_path):
    if stamp_or_path and os.path.isdir(stamp_or_path):
        return stamp_or_path
    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    candidates = sorted(glob(os.path.join(repo_root, "results", "chain_rw_*")))
    return candidates[-1] if candidates else None


def load_csv(path):
    df = pd.read_csv(path)
    if df["success_rate"].dtype == object:
        df["success_rate"] = df["success_rate"].astype(str).str.rstrip("%").astype(float)
    return df


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", help="Path to chain_rw_<stamp>/")
    parser.add_argument("--outdir", default="results/figs")
    args = parser.parse_args()

    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    root = find_root(args.root) if args.root else find_root(None)
    if not root:
        sys.exit("no chain_rw_*/ found under results/")
    stamp = os.path.basename(root).replace("chain_rw_", "")
    out_dir = os.path.join(repo_root, args.outdir)
    os.makedirs(out_dir, exist_ok=True)

    ratios = sorted(
        int(os.path.basename(d)[1:])
        for d in glob(os.path.join(root, "r*"))
        if os.path.isdir(d)
    )
    if not ratios:
        sys.exit(f"no r*/ dirs under {root}")

    # Load all CSVs into a dict[ratio] -> df.
    dfs = {}
    for r in ratios:
        csv = os.path.join(root, f"r{r}", "summary.csv")
        if not os.path.exists(csv):
            print(f"[warn] missing {csv}", file=sys.stderr); continue
        dfs[r] = load_csv(csv)

    # 2-panel figure: p99 vs throughput, throughput vs target_rate.
    fig, axes = plt.subplots(1, 2, figsize=(14, 5))
    fig.suptitle(
        f"chain read/write ratio sensitivity (vegeta open-loop, 15s/point)  —  {stamp}",
        fontsize=11)

    # ── Panel 1: p99 latency vs measured throughput ─────────────────────────
    ax = axes[0]
    for i, r in enumerate(ratios):
        df = dfs[r]
        for mode, palette in [("nocm", NOCM_PALETTE), ("flame", FLAME_PALETTE)]:
            sub = df[(df["mode"] == mode) & (df["success_rate"] >= 95)].sort_values("actual_rps")
            if sub.empty:
                continue
            xs = [0.0] + list(sub["actual_rps"])
            ys = [0.0] + list(sub["p99_secs"] * LAT_SCALE)
            ax.plot(xs, ys,
                    color=palette[ratios.index(r)],
                    linestyle="-" if mode == "nocm" else "--",
                    marker="o" if mode == "nocm" else "s",
                    markersize=4, linewidth=1.6,
                    label=f"{mode} r{r}")
    ax.set_xlabel("Throughput (req/s)")
    ax.set_ylabel("p99 latency (ms)")
    ax.set_title("p99 latency vs throughput")
    ax.grid(True, linestyle=":", alpha=0.6)
    ax.legend(fontsize=7, ncol=2, loc="upper left")
    ax.xaxis.set_major_formatter(
        ticker.FuncFormatter(lambda x, _: f"{x/1000:.0f}K" if x >= 1000 else f"{x:.0f}"))

    # ── Panel 2: peak rps + Redis CPU vs READ_PCT (bar / scatter) ────────────
    ax = axes[1]
    nocm_peak, flame_peak, nocm_redis, flame_redis = [], [], [], []
    for r in ratios:
        df = dfs[r]
        for mode, peak_list, redis_list in [
            ("nocm", nocm_peak, nocm_redis),
            ("flame", flame_peak, flame_redis),
        ]:
            sub = df[(df["mode"] == mode) & (df["success_rate"] >= 95)]
            if sub.empty:
                peak_list.append(0); redis_list.append(0); continue
            peak = sub["actual_rps"].max()
            peak_row = sub[sub["actual_rps"] == peak].iloc[0]
            peak_list.append(peak)
            redis_list.append(peak_row.get("redis_cpu_util", 0))
    width = 0.35
    x = list(range(len(ratios)))
    ax.bar([i - width/2 for i in x], nocm_peak, width=width,
           color="#4C72B0", label="nocm peak rps")
    ax.bar([i + width/2 for i in x], flame_peak, width=width,
           color="#DD8452", label="flame peak rps")
    ax.set_xticks(x)
    ax.set_xticklabels([f"{r}% read\n({r//10}r / {10-r//10}w)" for r in ratios])
    ax.set_ylabel("peak sustained rps (success ≥ 95%)")
    ax.set_title("Peak rps by read/write ratio")
    ax.grid(axis="y", linestyle=":", alpha=0.5)
    ax.legend(fontsize=8, loc="lower right")
    # secondary axis: redis CPU (cores) at peak
    ax2 = ax.twinx()
    ax2.plot(x, nocm_redis, color="#08306b", marker="o", linestyle=":",
             linewidth=1.5, label="nocm redis cores @peak")
    ax2.plot(x, flame_redis, color="#7f2704", marker="s", linestyle=":",
             linewidth=1.5, label="flame redis cores @peak")
    ax2.set_ylabel("redis CPU cores at peak")
    ax2.legend(fontsize=7, loc="upper right")

    fig.tight_layout(rect=[0, 0, 1, 0.94])
    out = os.path.join(out_dir, f"chain_rw_{stamp}.png")
    fig.savefig(out, dpi=200)
    print(f"saved {out}")


if __name__ == "__main__":
    main()
