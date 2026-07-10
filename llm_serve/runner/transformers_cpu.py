import time
from typing import Protocol

import torch
from transformers import AutoConfig, AutoModelForCausalLM
from mini_llm_serve.v1 import core_pb2, execute_pb2
from runner.base import ModelRunner

class QwenRunnerConfig(Protocol):
    @property
    def model_path(self) -> str: ...
    @property
    def dtype(self) -> str: ...

class QwenTransformersRunner(ModelRunner):
    def __init__(self, cfg: QwenRunnerConfig):
        self.cfg = cfg
        self.config = AutoConfig.from_pretrained(
            cfg.model_path,
            trust_remote_code=True,
        )

        self.dtype = resolve_torch_dtype(cfg.dtype)

        self.model = AutoModelForCausalLM.from_pretrained(
            cfg.model_path,
            dtype=self.dtype,
            device_map=None,
            trust_remote_code=True,
        )
        self.model.eval()
        self.eos_token_ids = normalize_eos_token_ids(self.config.eos_token_id)
    
    async def prefill(self, item) -> execute_pb2.ExecuteResult:
        start_time = time.perf_counter()

        execution_ms = int((time.perf_counter() - start_time) * 1000)
        return execute_pb2.ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=False,
            token_id=0,
            finish_reason=core_pb2.FINISH_REASON_UNSPECIFIED,
            computed_tokens=len(item.token_ids),
            generated_tokens=0,
            execution_ms=execution_ms,
            error_message="",
        )

    async def decode(self, item) -> execute_pb2.ExecuteResult:
        start_time = time.perf_counter()

        input_ids = torch.tensor([list(item.token_ids)], dtype=torch.long)
        
        with torch.inference_mode():
            outputs = self.model(input_ids=input_ids)
            logits = outputs.logits[:, -1, :]
            next_token_id = int(torch.argmax(logits, dim=-1).item())

        execution_ms = int((time.perf_counter() - start_time) * 1000)

        done = next_token_id in self.eos_token_ids

        finish_reason = (
            core_pb2.FINISH_REASON_STOP
            if done
            else core_pb2.FINISH_REASON_UNSPECIFIED
        )

        return execute_pb2.ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=done,
            token_id=next_token_id,
            finish_reason=finish_reason,
            computed_tokens=0,
            generated_tokens=1,
            execution_ms=execution_ms,
            error_message="",
        )

def normalize_eos_token_ids(eos):
    if eos is None:
        return set()
    if isinstance(eos, int):
        return {eos}
    return set(eos)

def resolve_torch_dtype(dtype: str) -> torch.dtype:
    match dtype:
        case "float32" | "fp32":
            return torch.float32
        case "float16" | "fp16":
            return torch.float16
        case "bfloat16" | "bf16":
            return torch.bfloat16
        case _:
            raise ValueError(f"unsupported dtype: {dtype}")