from executor_service import ExecuteServiceImpl
from runner.factory import create_runner
from setting import load_config
from transfer_http import create_executor_app


cfg = load_config()
runner = create_runner(cfg)
service = ExecuteServiceImpl(runner, cfg)
app = create_executor_app(service)
