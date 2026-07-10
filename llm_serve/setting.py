import tomllib
from dataclasses import dataclass

@dataclass(frozen=True)
class RunnerConfig:
    executor_id: str
    kind: str
    model_id: str
    model_path: str
    dtype: str = "float32"

@dataclass(frozen=True)
class RuntimeConfig:
    device: str
    tensor_parallel_size: int
    gpu_memory_utilization: float

@dataclass(frozen=True)
class ExecutorConfig:
    runner: RunnerConfig
    runtime: RuntimeConfig


def load_config(path: str = "config/executor.toml") -> ExecutorConfig:
    with open(path, "rb") as f:
        raw = tomllib.load(f)

    return ExecutorConfig(
        runner=RunnerConfig(**raw["runner"]),
        runtime=RuntimeConfig(**raw["runtime"]),
    )
