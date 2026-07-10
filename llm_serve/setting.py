import json
import tomllib
from dataclasses import dataclass
from pathlib import Path

@dataclass(frozen=True)
class RunnerConfig:
    executor_id: str
    model_id: str
    model_path: str
    model_type: str
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

    runner = raw["runner"]
    runner["model_type"] = read_model_type(runner["model_path"])

    return ExecutorConfig(
        runner=RunnerConfig(**runner),
        runtime=RuntimeConfig(**raw["runtime"]),
    )


def read_model_type(model_path: str) -> str:
    with open(Path(model_path) / "config.json", "rb") as f:
        config = json.load(f)

    model_type = config.get("model_type")
    if not model_type:
        raise ValueError(f"model config missing model_type: {model_path}")
    return model_type
