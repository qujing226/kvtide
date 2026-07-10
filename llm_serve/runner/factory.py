from runner.base import ModelRunner
from runner.mock import MockRunner
from runner.transformers_cpu import QwenTransformersRunner
from setting import ExecutorConfig


def create_runner(cfg: ExecutorConfig) -> ModelRunner:
    if cfg.runner.kind == "mock":
        return MockRunner()
    if cfg.runner.kind == "qwen":
        if cfg.runtime.device == "cpu":
            return QwenTransformersRunner(cfg.runner)
        if cfg.runtime.device == "cuda":
            raise ValueError("unsupported cuda runtime yet")
    # if cfg.kind == "cuda":
    #     return CUDAModelRunner(cfg)
    raise ValueError(f"unsupported runner kind: {cfg.runner.kind}")
