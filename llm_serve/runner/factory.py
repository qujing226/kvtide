from runner.base import ModelRunner, RunnerConfig
from runner.mock import MockRunner
from runner.transformers_cpu import CPUTransformersRunner


def create_runner(cfg: RunnerConfig) -> ModelRunner:
    if cfg.kind == "mock":
        return MockRunner()
    if cfg.kind == "cpu":
        return CPUTransformersRunner(cfg.model_path)
    # if cfg.kind == "cuda":
    #     return CUDAModelRunner(cfg)
    raise ValueError(f"unsupported runner kind: {cfg.kind}")
