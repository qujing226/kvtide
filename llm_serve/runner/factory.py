from runner.base import ModelRunner
from runner.mock import MockRunner
from runner.transformers_cpu import QwenTransformersRunner
from setting import ExecutorConfig


def create_runner(cfg: ExecutorConfig) -> ModelRunner:
    if cfg.runner.model_type == "mock":
        return MockRunner()
    if cfg.runner.model_type == "qwen3":
        if cfg.runtime.device == "cpu":
            return QwenTransformersRunner(cfg.runner)
        if cfg.runtime.device == "cuda":
            raise ValueError("unsupported cuda runtime yet")
    # if cfg.model_type == "cuda":
    #     return CUDAModelRunner(cfg)
    raise ValueError(f"unsupported runner model_type: {cfg.runner.model_type}")
