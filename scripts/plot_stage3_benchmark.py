from pathlib import Path
import os

os.environ.setdefault("MPLCONFIGDIR", "/tmp/matplotlib")

import matplotlib.pyplot as plt
from matplotlib.patches import Patch
import numpy as np


scenarios = ["cache_miss", "cache_hit", "mixed_prompt", "block_pressure"]
labels = ["Miss", "Hit", "Mixed", "Pressure"]
colors = ["#E97132", "#2E8B57", "#4F92D9", "#C43C39"]

throughput = [4.40, 4.90, 4.91, 2.38]
ttft = [1.6175, 0.8390, 1.3555, 2.1182]
avg_batch_size = [5.50, 5.95, 6.17, 3.38]
evictions = [4490, 460, 1475, 6164]

root = Path(__file__).resolve().parents[1]
output = root / "assets" / "Stage3_Benchmark_Summary.svg"

plt.rcParams["svg.fonttype"] = "path"
plt.style.use("seaborn-v0_8-whitegrid")
fig, axes = plt.subplots(2, 2, figsize=(13.5, 8.2))
fig.patch.set_facecolor("white")

x = np.arange(len(scenarios))


def draw_bars(ax, values, title, ylabel, value_fmt, ylim_pad=0.18):
    bars = ax.bar(x, values, color=colors, width=0.62, alpha=0.92)
    ax.set_title(title, fontsize=13, fontweight="bold", pad=14)
    ax.set_ylabel(ylabel, fontsize=10)
    ax.set_xticks(x)
    ax.set_xticklabels(labels, fontsize=10)
    ax.set_ylim(0, max(values) * (1 + ylim_pad))
    ax.grid(axis="y", alpha=0.35)
    ax.grid(axis="x", visible=False)
    for spine in ("top", "right"):
        ax.spines[spine].set_visible(False)
    for bar, value in zip(bars, values):
        ax.text(
            bar.get_x() + bar.get_width() / 2,
            bar.get_height() + max(values) * 0.03,
            value_fmt(value),
            ha="center",
            va="bottom",
            fontsize=9,
            fontweight="bold",
        )


draw_bars(
    axes[0, 0],
    ttft,
    "Time To First Token (TTFT)",
    "seconds",
    lambda v: f"{v:.4f}s",
)
draw_bars(
    axes[0, 1],
    throughput,
    "Throughput",
    "requests / second",
    lambda v: f"{v:.2f}",
)
draw_bars(
    axes[1, 0],
    avg_batch_size,
    "Average Batch Size",
    "work items / batch",
    lambda v: f"{v:.2f}",
)
draw_bars(
    axes[1, 1],
    evictions,
    "Prefix Cache Evictions",
    "evicted blocks",
    lambda v: f"{int(v):,}",
    ylim_pad=0.22,
)

fig.suptitle(
    "Stage 3 Benchmark: Prefix cache and KV block pressure",
    fontsize=16,
    fontweight="bold",
    y=0.97,
)
fig.text(
    0.5,
    0.055,
    "Workloads: cache/mixed = 1000 requests @ concurrency 100 | block pressure = 320 requests @ concurrency 32",
    fontsize=10,
    ha="center",
    color="#555555",
)
fig.text(
    0.5,
    0.025,
    "Signal: prefix cache lowers TTFT; block pressure increases eviction churn and reduces batch efficiency.",
    fontsize=10,
    ha="center",
    color="#1C2B39",
    fontweight="bold",
)

fig.legend(
    handles=[
        Patch(facecolor=color, edgecolor="none", alpha=0.92)
        for color in colors
    ],
    labels=["cache miss", "cache hit", "mixed prompt", "block pressure"],
    loc="lower center",
    ncol=4,
    frameon=False,
    bbox_to_anchor=(0.5, 0.08),
    fontsize=10,
)

fig.tight_layout(rect=(0, 0.13, 1, 0.93), h_pad=2.2, w_pad=2.0)
fig.savefig(str(output), format="svg", bbox_inches="tight")
print(f"saved to {output}")
