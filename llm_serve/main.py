from mini_llm_serve.v1 import executor_connect
from executor_service import ExecuteServiceImpl
from runner.factory import create_runner
from setting import load_config


cfg = load_config()
runner = create_runner(cfg)
app = executor_connect.ExecutorServiceASGIApplication(ExecuteServiceImpl(runner, cfg))
