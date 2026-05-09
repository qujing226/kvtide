from pathlib import Path
import os

os.environ.setdefault("MPLCONFIGDIR", "/tmp/matplotlib")

import matplotlib.pyplot as plt
import numpy as np


scenarios = ["cache_miss", "cache_hit", "mixed_prompt"]
labels = ["cache miss", "cache hit", "mixed prompt"]

ttft = [1.7322, 0.3250, 1.2117]
tbt = [0.4109, 0.3430, 0.3209]
throughput = [3.28, 4.10, 4.22]

root = Path(__file__).resolve().parents[1]
output = root / "assets" / "Stage2_Benchmark_Summary.svg"

plt.style.use("seaborn-v0_8-whitegrid")

fig, ax1 = plt.subplots(figsize=(10, 6))
ax2 = ax1.twinx()

x = np.arange(len(scenarios))
width = 0.25

throughput_bars = ax2.bar(
    x - width,
    throughput,
    width,
    label="throughput",
    color="#2ca02c",
    alpha=0.88,
)
ttft_bars = ax1.bar(
    x,
    ttft,
    width,
    label="TTFT",
    color="#1f77b4",
    alpha=0.92,
)
tbt_bars = ax1.bar(
    x + width,
    tbt,
    width,
    label="TBT",
    color="#ff7f0e",
    alpha=0.9,
)

ax1.set_title("Stage 2 Benchmark: Prefix Cache Metadata Impact", fontsize=15, pad=16)
ax1.set_ylabel("Latency (s)", fontsize=12)
ax2.set_ylabel("Throughput (req/s)", fontsize=12)

ax1.set_xticks(x)
ax1.set_xticklabels(labels)
ax1.set_ylim(0, 2.0)
ax2.set_ylim(0, 5.0)

for bars in (ttft_bars, tbt_bars):
    for bar in bars:
        height = bar.get_height()
        ax1.text(
            bar.get_x() + bar.get_width() / 2,
            height + 0.04,
            f"{height:.4f}s",
            ha="center",
            va="bottom",
            fontsize=9,
        )

for bar in throughput_bars:
    height = bar.get_height()
    ax2.text(
        bar.get_x() + bar.get_width() / 2,
        height + 0.10,
        f"{height:.2f}",
        ha="center",
        va="bottom",
        fontsize=9,
        color="#2ca02c",
    )

handles = [throughput_bars, ttft_bars, tbt_bars]
legend_labels = ["throughput", "TTFT", "TBT"]
ax1.legend(handles, legend_labels, loc="upper right", frameon=True)

fig.text(
    0.10,
    0.02,
    "1000 requests | concurrency 100 | prefix cache hit saved 147k prompt tokens",
    fontsize=10,
    color="#555555",
)

fig.tight_layout(rect=(0, 0.04, 1, 1))
fig.savefig(str(output), format="svg", bbox_inches="tight")
print(f"saved to {output}")
