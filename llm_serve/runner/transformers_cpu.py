import torch
from transformers import AutoModelForCausalLM
from mini_llm_serve.v1 import core_pb2, execute_pb2
from runner.base import ModelRunner

class CPUTransformersRunner(ModelRunner):
    def __init__(self, model_path: str):
        self.model = AutoModelForCausalLM.from_pretrained(
            model_path,
            torch_dtype= torch.float32,
            device_map=None,
            trust_remote_code=True,
        )
        self.model.eval()
    
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
        
        with torch.no_grad():
            outputs = self.model(input_ids=input_ids)
            logits = outputs.logits[:, -1, :]
            next_token_id = int(torch.argmax(logits, dim= -1).item())

        return execute_pb2.ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            done=False,
            token_id=next_token_id,
            finish_reason=core_pb2.FINISH_REASON_UNSPECIFIED,
            computed_tokens=0,
            generated_tokens=1,
            execution_ms=0,
            error_message="",
        )
