#!/usr/bin/env python3
"""
Plot the closed-loop oha sweep alongside the most recent open-loop mix
sweep for chain, so we can compare methodology side-by-side.

Inputs:
  results/chain_closedloop_<stamp>/summary.csv   (oha, columns: mode,concurrency,...,p50_secs,p99_secs,rps,success_rate)
  results/chain_mix_<stamp>/summary.csv           (vegeta, columns: mode,target_rate,actual_rps,...,p99_secs,success_rate)

Output: results/figs/chain_closed_vs_open_<stamp>.png
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


def latest(prefix):
    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    candidates = sorted(glob(os.path.join(repo_root, "results", f"{prefix}_*")))
    return candidates[-1] if candidates else None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--closed", help="path to chain_closedloop_<stamp>/")
    parser.add_argument("--open", dest="open_", help="path to chain_mix_<stamp>/")
    parser.add_argument("--outdir", default="results/figs")
    args = parser.parse_args()

    closed = args.closed or latest("chain_closedloop")
    opn    = args.open_ or latest("chain_mix")
    if not closed:
        sys.exit("no chain_closedloop_*/ found")
    if not opn:
        sys.exit("no chain_mix_*/ found")

    def _load(p):
        df = pd.read_csv(p)
        if df["success_rate"].dtype == object:
            df["success_rate"] = df["success_rate"].astype(str).str.rstrip("%").astype(float)
        return df
    df_c = _load(os.path.join(closed, "summary.csv"))
    df_o = _load(os.path.join(opn, "summary.csv"))
    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    out_dir = os.path.join(repo_root, args.outdir)
    os.makedirs(out_dir, exist_ok=True)

    fig, ax = plt.subplots(figsize=(10, 6))
    fig.suptitle(
        "chain: closed-loop (oha) vs open-loop (vegeta) — p99 latency vs throughput",
        fontsize=12)

    STYLE = {
        ("nocm",  "closed"):  dict(color="#08306b", marker="o", linestyle="-",  label="nocm — oha (closed-loop)"),
        ("flame", "closed"):  dict(color="#7f2704", marker="s", linestyle="-",  label="flame — oha (closed-loop)"),
        ("nocm",  "open"):    dict(color="#6baed6", marker="o", linestyle="--", label="nocm — vegeta (open-loop)"),
        ("flame", "open"):    dict(color="#fdae6b", marker="s", linestyle="--", label="flame — vegeta (open-loop)"),
    }

    for mode in ("nocm", "flame"):
        sub = df_c[df_c["mode"] == mode].sort_values("rps")
        if not sub.empty:
            xs = [0.0] + list(sub["rps"])
            ys = [0.0] + list(sub["p99_secs"] * LAT_SCALE)
            ax.plot(xs, ys, linewidth=2, markersize=6, **STYLE[(mode, "closed")])

    for mode in ("nocm", "flame"):
        sub = df_o[(df_o["mode"] == mode) & (df_o["success_rate"] >= 95)].sort_values("actual_rps")
        if not sub.empty:
            xs = [0.0] + list(sub["actual_rps"])
            ys = [0.0] + list(sub["p99_secs"] * LAT_SCALE)
            ax.plot(xs, ys, linewidth=1.6, markersize=5, **STYLE[(mode, "open")])

    # find x/y maxima from valid points only
    x_max = max(df_c["rps"].max(),
                df_o[df_o["success_rate"] >= 95]["actual_rps"].max())
    y_max = max(df_c["p99_secs"].max(),
                df_o[df_o["success_rate"] >= 95]["p99_secs"].max()) * LAT_SCALE

    ax.set_xlim(0, x_max * 1.05)
    ax.set_ylim(0, y_max * 1.10)
    ax.set_xlabel("Throughput (req/s)", fontsize=11)
    ax.set_ylabel("p99 latency (ms)", fontsize=11)
    ax.grid(True, linestyle=":", alpha=0.6)
    ax.xaxis.set_major_formatter(
        ticker.FuncFormatter(lambda x, _: f"{x/1000:.0f}K" if x >= 1000 else f"{x:.0f}"))
    ax.legend(fontsize=10, loc="upper left")

    stamp = os.path.basename(closed).replace("chain_closedloop_", "")
    fig.tight_layout(rect=[0, 0, 1, 0.95])
    out = os.path.join(out_dir, f"chain_closed_vs_open_{stamp}.png")
    fig.savefig(out, dpi=200)
    print(f"saved {out}")
    print(f"  closed-loop: {os.path.basename(closed)}")
    print(f"  open-loop:   {os.path.basename(opn)}")


if __name__ == "__main__":
    main()
