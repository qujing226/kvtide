TARGET ?= http://127.0.0.1:8800
METRICS_URL ?= http://127.0.0.1:8801/metrics
INGRESS_NGINX_MANIFEST ?= https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.15.1/deploy/static/provider/cloud/deploy.yaml

run:
	go run ./cmd/server/. --conf="server.toml"

bench-quick:
	go run ./cmd/bench --profile quick --target $(TARGET) --metrics-url $(METRICS_URL)

bench-report:
	go run ./cmd/bench --profile report --target $(TARGET) --metrics-url $(METRICS_URL)

docker-build-server:
	docker build -f docker/server.Dockerfile -t mini-llm-server:local .

docker-build-executor:
	docker build -f docker/executor.Dockerfile -t mini-llm-executor:local .

docker-build: docker-build-server docker-build-executor

docker-run-executor:
	docker run --rm -p 19991:19991 mini-llm-executor:local

docker-run-server:
	docker run --rm --network host -v "$(PWD)/server.toml:/etc/mini-llm/server.toml:ro" mini-llm-server:local --conf=/etc/mini-llm/server.toml

kind-create:
	kind create cluster --name mini-llm --config k8s/kind/cluster.yaml

kind-load-images:
	kind load docker-image mini-llm-server:local --name mini-llm
	kind load docker-image mini-llm-executor:local --name mini-llm

k8s-render:
	kubectl kustomize k8s/base

k8s-install-ingress-nginx:
	kubectl apply -f $(INGRESS_NGINX_MANIFEST)
	kubectl -n ingress-nginx rollout status deploy/ingress-nginx-controller

k8s-apply:
	kubectl apply -k k8s/base

k8s-status:
	kubectl -n mini-llm get pods,deploy,svc,ingress -o wide

k8s-rollout:
	kubectl -n mini-llm rollout status deploy/mock-executor
	kubectl -n mini-llm rollout status deploy/demo-server

k8s-port-forward-admin:
	kubectl -n mini-llm port-forward svc/demo-server 8801:8801

k8s-delete:
	kubectl delete -k k8s/base --ignore-not-found
