TARGET ?= http://127.0.0.1:8800
METRICS_URL ?= http://127.0.0.1:8801/metrics

run:
	go run ./cmd/server/. --conf="server.toml"

bench-quick:
	go run ./cmd/bench --profile quick --target $(TARGET) --metrics-url $(METRICS_URL)

bench-report:
	go run ./cmd/bench --profile report --target $(TARGET) --metrics-url $(METRICS_URL)

docker-build:
	docker build -f docker/server.Dockerfile -t kvtide-server:local .
	docker build -f docker/executor.Dockerfile -t kvtide-executor:local .
	docker build -f docker/web.Dockerfile -t kvtide-web:local .

docker-save:
	docker save -o deploy/kvtide-server.tar kvtide-server:local
	docker save -o deploy/kvtide-executor.tar kvtide-executor:local
	docker save -o deploy/kvtide-web.tar kvtide-web:local

.PHONY: test stress-test web-dev web-test web-build docker-up-prod kube-start kube-apply kube-down kube-forward

test:
	go test ./...

stress-test:
	go test -tags=stress ./tests -run Stress -count=1

web-dev:
	cd web && npm run dev

web-test:
	cd web && npm run test:run

web-build:
	cd web && npm run build

docker-up-prod:
	docker compose -f docker-compose.yaml -f docker-compose.prod.yaml up --build -d

kube-start:
	kind create cluster --name kvtide --config k8s/kind/cluster.yaml
	kind load docker-image kvtide-server:local kvtide-executor:local \
	  --name kvtide
	kubectl apply -k k8s/base
	kubectl rollout status deployment/executor -n kvtide
	kubectl rollout status deployment/server -n kvtide

kube-apply:
	kind load docker-image kvtide-server:local kvtide-executor:local \
	  --name kvtide
	kubectl apply -k k8s/base
	kubectl rollout restart deployment/executor deployment/server -n kvtide
	kubectl rollout status deployment/executor -n kvtide
	kubectl rollout status deployment/server -n kvtide

kube-down:
	kind delete cluster --name kvtide

kube-forward:
	kubectl port-forward -n kvtide service/server 8800:8800 8801:8801

llama-server:
	llama-server \
	-hf Qwen/Qwen2.5-1.5B-Instruct-GGUF:Q4_K_M \
	-c 2048 -t 8 -tb 8 \
	--metrics \
	--host 127.0.0.1 --port 8080

llama-cli:
	llama-cli \
	-hf Qwen/Qwen2.5-1.5B-Instruct-GGUF:Q4_K_M \
	-c 2048 -t 8 \
	-n 128 -cnv
