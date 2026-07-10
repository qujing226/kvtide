import torch
from transformers import AutoConfig, AutoModelForCausalLM
from mini_llm_serve.v1 import core_pb2, execute_pb2
from runner.base import ModelRunner

from typing import Protocol

class QwenRunnerConfig(Protocol):
    model_path: str
    dtype: str

class QwenTransformersRunner(ModelRunner):
    def __init__(self, cfg: QwenRunnerConfig):
        self.cfg = cfg
        self.config = AutoConfig.from_pretrained(
            cfg.model_path,
            trust_remote_code = True,
        )
        self.model = AutoModelForCausalLM.from_pretrained(
            cfg.model_path,
            dtype= torch.float32,
            device_map=None,
            trust_remote_code=True,
        )
        self.model.eval()
        self.eos_token_ids = normalize_eos_token_ids(self.config.eos_token_id)
    
    async def prefill(self, item) -> execute_pb2.ExecuteResult:
        return execute_pb2.ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=False,
            token_id=0,
            finish_reason=core_pb2.FINISH_REASON_UNSPECIFIED,
            computed_tokens=len(item.token_ids),
            generated_tokens=0,
            execution_ms=0,
            error_message="",
        )

    async def decode(self, item) -> execute_pb2.ExecuteResult:
        input_ids = torch.tensor([list(item.token_ids)], dtype=torch.long)
        
        with torch.inference_mode():
            outputs = self.model(input_ids=input_ids)
            logits = outputs.logits[:, -1, :]
            next_token_id = int(torch.argmax(logits, dim= -1).item())

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
            execution_ms=0,
            error_message="",
        )

def normalize_eos_token_ids(eos):
    if eos is None:
        return set()
    if isinstance(eos, int):
        return {eos}
    return set(eos)