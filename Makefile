TARGET ?= http://127.0.0.1:8800
METRICS_URL ?= http://127.0.0.1:8801/metrics

run:
	go run ./cmd/server/. --conf="server.toml"

bench-quick:
	go run ./cmd/bench --profile quick --target $(TARGET) --metrics-url $(METRICS_URL)

bench-report:
	go run ./cmd/bench --profile report --target $(TARGET) --metrics-url $(METRICS_URL)

docker-build:
	docker build -f docker/server.Dockerfile -t mini-llm-server:local .
	docker build -f docker/executor.Dockerfile -t mini-llm-executor:local .
	docker build -f docker/web.Dockerfile -t mini-llm-web:local .

docker-save:
	docker save -o deploy/mini-llm-server.tar mini-llm-server:local
	docker save -o deploy/mini-llm-executor.tar mini-llm-executor:local
	docker save -o deploy/mini-llm-web.tar mini-llm-web:local

.PHONY: test stress-test web-dev web-test web-build kube-start kube-apply kube-down kube-forward

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

kube-start:
	kind create cluster --name mini-llm --config k8s/kind/cluster.yaml
	kind load docker-image mini-llm-server:local mini-llm-executor:local \
	  --name mini-llm
	kubectl apply -k k8s/base
	kubectl rollout status deployment/executor -n mini-llm
	kubectl rollout status deployment/server -n mini-llm

kube-apply:
	kind load docker-image mini-llm-server:local mini-llm-executor:local \
	  --name mini-llm
	kubectl apply -k k8s/base
	kubectl rollout restart deployment/executor deployment/server -n mini-llm
	kubectl rollout status deployment/executor -n mini-llm
	kubectl rollout status deployment/server -n mini-llm

kube-down:
	kind delete cluster --name mini-llm

kube-forward:
	kubectl port-forward -n mini-llm service/server 8800:8800 8801:8801
