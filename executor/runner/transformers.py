from typing import cast

import torch
from adapter import DynamicCacheAdapter
from kvtide.v1 import core_pb2, executor_pb2
from runtime import BatchBuilder, PagedKVCache
from runtime.kv_transfer import KVTransferRuntime
from setting import RunnerConfig, RuntimeConfig
from transformers import AutoConfig, AutoModelForCausalLM, PreTrainedModel

from runner.base import ModelRunner, RuntimeInfo
from runner.device import (
    create_execution_timer,
    memory_snapshot,
    plan_kv_cache,
    resolve_runtime_config,
)

KV_CACHE_BLOCK_SIZE = 16


class Runner(ModelRunner):
    def __init__(
        self,
        runner_cfg: RunnerConfig,
        runtime_cfg: RuntimeConfig,
    ):
        self.runner_cfg = runner_cfg
        self.runtime_cfg = runtime_cfg
        self.model_config = AutoConfig.from_pretrained(
            runner_cfg.model_path,
            trust_remote_code=True,
        )
        self.dtype, self.device = resolve_runtime_config(
            runner_cfg.dtype, runtime_cfg.device
        )
        self.model: PreTrainedModel = AutoModelForCausalLM.from_pretrained(
            runner_cfg.model_path,
            dtype=self.dtype,
            device_map=None,
            trust_remote_code=True,
        )
        before_model = memory_snapshot(self.device)
        # load model
        self.model.eval()
        cast(torch.nn.Module, self.model).to(self.device)
        if self.device.type == "cuda":
            torch.cuda.empty_cache()
        after_model = memory_snapshot(self.device)

        self.block_size = KV_CACHE_BLOCK_SIZE
        self.num_layers = self.model_config.num_hidden_layers
        self.num_kv_heads = self.model_config.num_key_value_heads
        self.head_dim = getattr(
            self.model_config,
            "head_dim",
            self.model_config.hidden_size // self.model_config.num_attention_heads,
        )

        from runtime.kv_cache import cache_block_bytes

        block_bytes = cache_block_bytes(
            num_layers=self.num_layers,
            block_size=self.block_size,
            num_kv_heads=self.num_kv_heads,
            head_dim=self.head_dim,
            dtype=self.dtype,
        )

        plan = plan_kv_cache(
            device=self.device,
            before_model=before_model,
            after_model=after_model,
            gpu_memory_utilization=runtime_cfg.gpu_memory_utilization,
            kv_cache_memory_bytes=runtime_cfg.kv_cache_memory_bytes,
            cache_block_bytes=block_bytes,
        )

        self.num_kv_blocks = plan.num_blocks

        self.batch_builder = BatchBuilder(self.block_size)

        # if oom
        try:
            self.kv_cache = PagedKVCache(
                num_layers=self.num_layers,
                num_blocks=self.num_kv_blocks,
                block_size=self.block_size,
                num_kv_heads=self.num_kv_heads,
                head_dim=self.head_dim,
                dtype=self.dtype,
                device=self.device,
            )
        except torch.OutOfMemoryError as exc:
            if self.device.type != "cuda":
                raise

            raise RuntimeError(
                "CUDA OOM while allocating KV cache: "
                f"device={self.device}, "
                f"requested_bytes={plan.cache_bytes}, "
                f"num_blocks={plan.num_blocks}, "
                f"free_before_allocation={after_model.available_bytes}, "
                f"total={after_model.total_bytes}"
            ) from exc

        self.cache_adapter = DynamicCacheAdapter(
            paged_cache=self.kv_cache,
            model_config=self.model_config,
        )

        self._runtime_info = RuntimeInfo(
            model_type=self.model_config.model_type,
            dtype=str(self.dtype).removeprefix("torch."),
            block_size=self.block_size,
            num_kv_blocks=self.num_kv_blocks,
            num_hidden_layers=self.num_layers,
            num_kv_heads=self.num_kv_heads,
            head_dim=self.head_dim,
            total_memory_bytes=after_model.total_bytes,
            available_memory_bytes=after_model.available_bytes,
            kv_cache_bytes=self.kv_cache.cache_bytes,
        )

        self.eos_token_ids = normalize_eos_token_ids(self.model_config.eos_token_id)

        self._kv_transfer = KVTransferRuntime(
            model=cast(torch.nn.Module, self.model),
            model_config=self.model.config.to_dict(),
            cache=self.kv_cache,
            tensor_parallel_size=runtime_cfg.tensor_parallel_size,
        )

        self.timer = create_execution_timer(self.device)

    async def execute(
        self,
        items: list[executor_pb2.ExecuteItem],
    ) -> list[executor_pb2.ExecuteResult]:
        if not items:
            return []

        batch = self.batch_builder.build(items)

        allocated_blocks = sorted(
            {
                int(block_id)
                for item in batch.items
                for block_id in item.kv_blocks.allocated_blocks
            }
        )
        self.kv_cache.release(allocated_blocks)

        results: list[executor_pb2.ExecuteResult] = []

        for item_index, item in enumerate(batch.items):
            query_start = batch.query_start_locs[item_index]
            query_end = batch.query_start_locs[item_index + 1]

            input_ids = batch.input_ids[query_start:query_end]
            positions = batch.positions[query_start:query_end]
            slot_mapping = batch.slot_mapping[query_start:query_end]

            results.append(
                self._execute_one(
                    item=item,
                    input_ids=input_ids,
                    positions=positions,
                    slot_mapping=slot_mapping,
                )
            )

        return results

    def _execute_one(
        self,
        item: executor_pb2.ExecuteItem,
        input_ids: list[int],
        positions: list[int],
        slot_mapping: list[int],
    ) -> executor_pb2.ExecuteResult:
        self.timer.start()

        past_key_values = self.cache_adapter.build(
            block_table=list(item.kv_blocks.block_table),
            past_len=item.computed_tokens,
        )

        input_tensor = torch.tensor(
            [input_ids],
            dtype=torch.long,
            device=self.device,
        )
        position_tensor = torch.tensor(
            [positions],
            dtype=torch.long,
            device=self.device,
        )

        with torch.inference_mode():
            outputs = self.model(
                input_ids=input_tensor,
                position_ids=position_tensor,
                past_key_values=past_key_values,
                use_cache=True,
                # each sequence only need the last postion logits.
                logits_to_keep=1,
            )

        # DynamicCache has been appended with the current round's K/V
        # in-place for each layer of Attention.
        self.cache_adapter.write_new(
            cache=past_key_values,
            slot_mapping=slot_mapping,
        )

        token_id = 0
        generated_tokens = 0
        done = False
        finish_reason = core_pb2.FINISH_REASON_UNSPECIFIED

        if item.sample:
            logits = outputs.logits[:, -1, :]
            token_id = int(torch.argmax(logits, dim=-1).item())
            generated_tokens = 1
            done = token_id in self.eos_token_ids

            if done:
                finish_reason = core_pb2.FINISH_REASON_STOP

        computed_tokens = (
            item.num_new_tokens if item.phase == core_pb2.WORK_PHASE_PREFILL else 0
        )

        execution_ms = int(self.timer.stop_ms())

        return executor_pb2.ExecuteResult(
            work_id=item.work_id,
            request_id=item.request_id,
            token_id=token_id,
            done=done,
            finish_reason=finish_reason,
            computed_tokens=computed_tokens,
            generated_tokens=generated_tokens,
            execution_ms=execution_ms,
            error_message="",
        )

    async def release_blocks(self, block_ids: list[int]) -> None:
        self.kv_cache.release(block_ids)

    @property
    def runtime_info(self) -> RuntimeInfo:
        return self._runtime_info


def normalize_eos_token_ids(eos) -> set[int]:
    if eos is None:
        return set()
    if isinstance(eos, int):
        return {eos}
    return set(eos)
