import argparse
from pathlib import Path
import sys

import uvicorn


sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from executor_service import ExecuteServiceImpl
from runner.factory import create_runner
from setting import ExecutorConfig, RunnerConfig, RuntimeConfig, read_model_type
from transfer_http import create_executor_app


def parse_args():
    parser = argparse.ArgumentParser(
        description="Run one KVTide executor for the single-GPU KV transfer test."
    )
    parser.add_argument("--executor-id", required=True)
    parser.add_argument("--model-path", required=True)
    parser.add_argument("--port", required=True, type=int)
    parser.add_argument("--model-id", default="Qwen/Qwen3-0.6B")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--device", default="cuda:0")
    parser.add_argument("--dtype", default="bfloat16")
    parser.add_argument("--kv-cache-memory-bytes", type=int, default=536870912)
    return parser.parse_args()


def main():
    args = parse_args()
    cfg = ExecutorConfig(
        runner=RunnerConfig(
            executor_id=args.executor_id,
            model_id=args.model_id,
            model_path=args.model_path,
            model_type=read_model_type(args.model_path),
            dtype=args.dtype,
        ),
        runtime=RuntimeConfig(
            device=args.device,
            tensor_parallel_size=1,
            gpu_memory_utilization=0.9,
            kv_cache_memory_bytes=args.kv_cache_memory_bytes,
            transfer_endpoint=f"http://127.0.0.1:{args.port}",
        ),
    )
    service = ExecuteServiceImpl(create_runner(cfg), cfg)
    uvicorn.run(
        create_executor_app(service),
        host=args.host,
        port=args.port,
    )


if __name__ == "__main__":
    main()
