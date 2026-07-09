import tomllib
from dataclasses import dataclass
from runner.base import RunnerConfig

@dataclass(frozen=True)
class ExecutorConfig:
    runner: RunnerConfig

def load_config(path: str = "config/executor.toml") -> ExecutorConfig:
    with open(path, "rb") as f:
        raw = tomllib.load(f)

    return ExecutorConfig(
        runner=RunnerConfig(**raw["runner"]),
    )
