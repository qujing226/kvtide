TARGET ?= http://127.0.0.1:8800
METRICS_URL ?= http://127.0.0.1:8801/metrics
DOCKER ?= docker
KIND ?= kind
KUBECTL ?= kubectl
KIND_CLUSTER ?= mini-llm
SERVER_IMAGE ?= mini-llm-server:local
EXECUTOR_IMAGE ?= mini-llm-executor:local
INGRESS_NGINX_MANIFEST ?= https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.15.1/deploy/static/provider/cloud/deploy.yaml

run:
	go run ./cmd/server/. --conf="server.toml"

bench-smoke:
	go run ./cmd/bench --mode smoke --target $(TARGET) --metrics-url $(METRICS_URL)

bench-cache-miss:
	go run ./cmd/bench --mode cache_miss --target $(TARGET) --metrics-url $(METRICS_URL)

bench-cache-hit:
	go run ./cmd/bench --mode cache_hit --target $(TARGET) --metrics-url $(METRICS_URL)

bench-mixed-prompt:
	go run ./cmd/bench --mode mixed_prompt --target $(TARGET) --metrics-url $(METRICS_URL)

docker-build-server:
	$(DOCKER) build -f docker/server.Dockerfile -t $(SERVER_IMAGE) .

docker-build-executor:
	$(DOCKER) build -f docker/executor.Dockerfile -t $(EXECUTOR_IMAGE) .

docker-build: docker-build-server docker-build-executor

docker-run-executor:
	$(DOCKER) run --rm -p 19991:19991 $(EXECUTOR_IMAGE)

docker-run-server:
	$(DOCKER) run --rm --network host -v "$(PWD)/server.toml:/etc/mini-llm/server.toml:ro" $(SERVER_IMAGE) --conf=/etc/mini-llm/server.toml

kind-create:
	$(KIND) create cluster --name $(KIND_CLUSTER) --config k8s/kind/cluster.yaml

kind-load-images:
	$(KIND) load docker-image $(SERVER_IMAGE) --name $(KIND_CLUSTER)
	$(KIND) load docker-image $(EXECUTOR_IMAGE) --name $(KIND_CLUSTER)

k8s-render:
	$(KUBECTL) kustomize k8s/base

k8s-install-ingress-nginx:
	$(KUBECTL) apply -f $(INGRESS_NGINX_MANIFEST)
	$(KUBECTL) -n ingress-nginx rollout status deploy/ingress-nginx-controller

k8s-apply:
	$(KUBECTL) apply -k k8s/base

k8s-status:
	$(KUBECTL) -n mini-llm get pods,deploy,svc,ingress -o wide

k8s-rollout:
	$(KUBECTL) -n mini-llm rollout status deploy/mock-executor
	$(KUBECTL) -n mini-llm rollout status deploy/demo-server

k8s-port-forward-admin:
	$(KUBECTL) -n mini-llm port-forward svc/demo-server 8801:8801

k8s-delete:
	$(KUBECTL) delete -k k8s/base --ignore-not-found
