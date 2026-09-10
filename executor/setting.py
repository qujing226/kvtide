import json
from dataclasses import dataclass
from pathlib import Path

import tomllib
from urllib.parse import urlsplit


@dataclass(frozen=True)
class RunnerConfig:
    executor_id: str
    model_id: str
    model_path: str
    model_type: str
    dtype: str = "auto"


@dataclass(frozen=True)
class RuntimeConfig:
    device: str
    tensor_parallel_size: int
    gpu_memory_utilization: float
    kv_cache_memory_bytes: int
    # Empty means no advertised peer endpoint; do not guess a reachable address.
    transfer_endpoint: str = ""

    def __post_init__(self):
        if self.kv_cache_memory_bytes < 0:
            raise ValueError("kv_cache_memory_bytes must not be negative")
        if self.device == "cpu" and self.kv_cache_memory_bytes == 0:
            raise ValueError("CPU runtime requires a positive KV cache budget")
        if self.transfer_endpoint:
            endpoint = urlsplit(self.transfer_endpoint)
            if (
                endpoint.scheme not in {"http", "https"}
                or not endpoint.hostname
                or endpoint.hostname in {"0.0.0.0", "::"}
                or endpoint.username is not None
                or endpoint.password is not None
                or endpoint.query or endpoint.fragment
            ):
                raise ValueError("transfer_endpoint must be a peer-reachable HTTP(S) base URL")
            # Accessing port also validates its syntax and range.
            _ = endpoint.port


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
