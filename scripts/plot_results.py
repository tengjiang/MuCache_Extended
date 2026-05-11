#!/usr/bin/env python3
"""
Plot throughput-latency sweep results for chain, boutique, and hotel benchmarks.

Usage:
    python3 scripts/plot_results.py
    python3 scripts/plot_results.py --outdir results/figs

Reads:
    results/chain_distributed/summary.csv
    results/boutique_distributed/summary.csv
    results/hotel_distributed/summary.csv

Outputs (in --outdir):
    throughput.png        — RPS vs concurrency, 3-panel (one per benchmark)
    latency_p50.png       — p50 latency vs concurrency, 3-panel
    latency_p99.png       — p99 latency vs concurrency, 3-panel
    combined.png          — 3×3 grid: all metrics × all benchmarks
    tput_latency.png      — latency vs throughput, 3-panel (p50 + p99 per benchmark)
"""

import argparse
import os
import sys

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.ticker as ticker
import pandas as pd

# ── colour / style ─────────────────────────────────────────────────────────────
STYLE = {
    "nocm":  dict(color="#4C72B0", marker="o", linestyle="-",  linewidth=2, markersize=6, label="http"),
    "flame": dict(color="#DD8452", marker="s", linestyle="--", linewidth=2, markersize=6, label="flame"),
}

BENCHMARKS = [
    ("chain",    "Chain (/ro_read, 5-hop)",        "results/chain_distributed/summary.csv"),
    ("boutique", "Boutique (/ro_home, 6-service)",  "results/boutique_distributed/summary.csv"),
    ("hotel",    "Hotel (/ro_search_hotels)",       "results/hotel_distributed/summary.csv"),
]

# latency columns are in seconds; convert to ms for display
LAT_SCALE = 1000   # s → ms


def load(path):
    df = pd.read_csv(path)
    # normalise column names (old files had p50_ms / p50_secs)
    df.columns = [
        c.replace("_ms", "_secs").replace("p50_secs", "p50_secs")
         .replace("p95_secs", "p95_secs").replace("p99_secs", "p99_secs")
        for c in df.columns
    ]
    for col in ("p50_secs", "p95_secs", "p99_secs"):
        if col not in df.columns:
            # try _ms variant and convert
            alt = col.replace("_secs", "_ms")
            if alt in df.columns:
                df[col] = df[alt] / 1000.0
    return df


def add_speedup_annotation(ax, df_nocm, df_flame, y_col, scale=1.0):
    """Annotate the highest-concurrency point with the flame/nocm ratio."""
    c_max = df_nocm["concurrency"].max()
    v_nocm  = df_nocm.loc[df_nocm["concurrency"] == c_max, y_col].values[0] * scale
    v_flame = df_flame.loc[df_flame["concurrency"] == c_max, y_col].values[0] * scale
    ratio = v_flame / v_nocm
    ax.annotate(
        f"×{ratio:.2f}",
        xy=(c_max, v_flame * scale if scale == 1.0 else v_flame),
        xytext=(8, 4), textcoords="offset points",
        fontsize=8, color=STYLE["flame"]["color"], fontweight="bold",
    )


def plot_metric(axes_row, dfs, y_col, ylabel, scale=1.0, annotate=True):
    for ax, (key, title, _), df in zip(axes_row, BENCHMARKS, dfs):
        for mode in ("nocm", "flame"):
            sub = df[df["mode"] == mode].sort_values("concurrency")
            ax.plot(sub["concurrency"], sub[y_col] * scale, **STYLE[mode])
        ax.set_title(title, fontsize=10, fontweight="bold")
        ax.set_xlabel("Concurrency", fontsize=9)
        ax.set_ylabel(ylabel, fontsize=9)
        ax.xaxis.set_major_locator(ticker.MultipleLocator(25))
        ax.grid(True, linestyle=":", alpha=0.6)
        ax.tick_params(labelsize=8)
        if annotate:
            df_n = df[df["mode"] == "nocm"]
            df_f = df[df["mode"] == "flame"]
            add_speedup_annotation(ax, df_n, df_f, y_col, scale)


def make_figure(outdir):
    os.makedirs(outdir, exist_ok=True)

    dfs = []
    for key, title, relpath in BENCHMARKS:
        csv = os.path.join(os.path.dirname(__file__), "..", relpath)
        csv = os.path.normpath(csv)
        if not os.path.exists(csv):
            print(f"[warn] missing {csv} — skipping {key}", file=sys.stderr)
            dfs.append(pd.DataFrame())
        else:
            dfs.append(load(csv))

    # ── 1. Throughput ──────────────────────────────────────────────────────────
    fig, axes = plt.subplots(1, 3, figsize=(13, 4), sharey=False)
    fig.suptitle("Throughput: flame vs nocm (distributed, N_REQUESTS=100 000)", fontsize=11)
    plot_metric(axes, dfs, "rps", "Requests / sec", scale=1.0)
    axes[0].legend(fontsize=8)
    fig.tight_layout()
    out = os.path.join(outdir, "throughput.png")
    fig.savefig(out, dpi=150)
    print(f"saved {out}")
    plt.close(fig)

    # ── 2. p50 latency ────────────────────────────────────────────────────────
    fig, axes = plt.subplots(1, 3, figsize=(13, 4), sharey=False)
    fig.suptitle("p50 Latency: flame vs nocm (distributed, N_REQUESTS=100 000)", fontsize=11)
    plot_metric(axes, dfs, "p50_secs", "p50 latency (ms)", scale=LAT_SCALE, annotate=False)
    axes[0].legend(fontsize=8)
    fig.tight_layout()
    out = os.path.join(outdir, "latency_p50.png")
    fig.savefig(out, dpi=150)
    print(f"saved {out}")
    plt.close(fig)

    # ── 3. p99 latency ────────────────────────────────────────────────────────
    fig, axes = plt.subplots(1, 3, figsize=(13, 4), sharey=False)
    fig.suptitle("p99 Latency: flame vs nocm (distributed, N_REQUESTS=100 000)", fontsize=11)
    plot_metric(axes, dfs, "p99_secs", "p99 latency (ms)", scale=LAT_SCALE, annotate=False)
    axes[0].legend(fontsize=8)
    fig.tight_layout()
    out = os.path.join(outdir, "latency_p99.png")
    fig.savefig(out, dpi=150)
    print(f"saved {out}")
    plt.close(fig)

    # ── 4. Combined 3×3 ──────────────────────────────────────────────────────
    fig, axes = plt.subplots(3, 3, figsize=(13, 11))
    fig.suptitle("MuCache distributed benchmark: flame vs nocm\n(3 benchmarks × throughput / p50 / p99)",
                 fontsize=12, fontweight="bold")

    metrics = [
        ("rps",      "Requests / sec",   1.0),
        ("p50_secs", "p50 latency (ms)", LAT_SCALE),
        ("p99_secs", "p99 latency (ms)", LAT_SCALE),
    ]
    for row_idx, (y_col, ylabel, scale) in enumerate(metrics):
        plot_metric(axes[row_idx], dfs, y_col, ylabel, scale=scale,
                    annotate=(row_idx == 0))

    # single legend in top-left
    axes[0][0].legend(fontsize=8)

    fig.tight_layout(rect=[0, 0, 1, 0.95])
    out = os.path.join(outdir, "combined.png")
    fig.savefig(out, dpi=150)
    print(f"saved {out}")
    plt.close(fig)


def plot_tput_latency(outdir, dfs):
    """Latency (p50 + p99) vs throughput — the classic systems curve.

    Both axes are anchored at (0, 0): each curve gets a synthetic (0, 0)
    anchor and axes are forced to start at the origin so the plot
    accurately conveys "no load → no latency, no throughput."
    """
    fig, axes = plt.subplots(1, 3, figsize=(15, 5), sharey=False)
    fig.suptitle("Latency vs Throughput: flame vs http  (distributed)",
                 fontsize=11)

    for ax, (key, title, _), df in zip(axes, BENCHMARKS, dfs):
        if df.empty:
            continue
        x_max = 0.0
        y_max = 0.0
        for mode in ("nocm", "flame"):
            sub = df[df["mode"] == mode].sort_values("rps")
            if sub.empty:
                continue
            style = STYLE[mode]
            xs = [0.0] + list(sub["rps"])
            p50 = [0.0] + list(sub["p50_secs"] * LAT_SCALE)
            p99 = [0.0] + list(sub["p99_secs"] * LAT_SCALE)
            x_max = max(x_max, max(xs))
            y_max = max(y_max, max(p99))
            ax.plot(xs, p50,
                    label=f"{'http' if mode=='nocm' else 'flame'} p50",
                    color=style["color"], marker=style["marker"],
                    linestyle="-", linewidth=2, markersize=6)
            ax.plot(xs, p99,
                    label=f"{'http' if mode=='nocm' else 'flame'} p99",
                    color=style["color"], marker=style["marker"],
                    linestyle=":", linewidth=1.5, markersize=5, alpha=0.55)

        ax.set_title(title, fontsize=10, fontweight="bold")
        ax.set_xlabel("Throughput (req/s)", fontsize=9)
        ax.set_ylabel("Latency (ms)", fontsize=9)
        ax.grid(True, linestyle=":", alpha=0.6)
        ax.tick_params(labelsize=8)
        # Force (0,0) origin with a small headroom margin (5%).
        ax.set_xlim(0, x_max * 1.05 if x_max > 0 else 1)
        ax.set_ylim(0, y_max * 1.10 if y_max > 0 else 1)
        ax.xaxis.set_major_formatter(
            ticker.FuncFormatter(lambda x, _: f"{x/1000:.0f}K" if x >= 1000 else f"{x:.0f}")
        )

    # legend only on first panel — 2 columns: nocm / flame
    handles, labels = axes[0].get_legend_handles_labels()
    axes[0].legend(handles, labels, fontsize=8, ncol=2,
                   loc="upper left", framealpha=0.9,
                   title="— p50  ··· p99", title_fontsize=8)

    fig.tight_layout()
    out = os.path.join(outdir, "tput_latency.png")
    fig.savefig(out, dpi=200)
    print(f"saved {out}")
    plt.close(fig)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--outdir", default="results/figs",
                        help="Output directory for PNG files (default: results/figs)")
    args = parser.parse_args()

    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    outdir = os.path.join(repo_root, args.outdir)
    make_figure(outdir)

    # reload dfs for the tput-latency plot
    dfs = []
    for key, title, relpath in BENCHMARKS:
        csv = os.path.join(repo_root, relpath)
        dfs.append(load(csv) if os.path.exists(csv) else pd.DataFrame())
    plot_tput_latency(outdir, dfs)
