# Kubernetes Deployment

This directory contains a local Kubernetes deployment for Mini LLM Serve using
[kind](https://kind.sigs.k8s.io/).

The deployment intentionally uses a one-to-one execution topology:

```text
host client
  -> kubectl port-forward
  -> server ClusterIP Service
  -> Go server Pod
  -> executor ClusterIP Service
  -> Python mock executor Pod
```

The executor has one replica because the current control plane models one
logical executor and one KV block pool. Increasing the Deployment replica count
would only add Kubernetes Service endpoints; it would not create independent
KV-aware inference workers.

## Prerequisites

- Docker
- kind
- kubectl
- local images:
  - `mini-llm-server:local`
  - `mini-llm-executor:local`

Build the images:

```bash
make docker-build
```

## Start

Create the three-node kind cluster, load both local images, apply all manifests,
and wait for both Deployments:

```bash
make kube-start
```

The cluster contains one control-plane node and two worker nodes. Kubernetes
chooses which worker runs each Pod.

## Verify

```bash
kubectl config current-context
kubectl get nodes -o wide
kubectl get pods,deploy,svc -n mini-llm -o wide
kubectl get endpointslices -n mini-llm
```

Expected context:

```text
kind-mini-llm
```

The `executor` and `server` EndpointSlices should each contain one ready Pod IP.

## Access

Forward the inference and metrics ports:

```bash
make kube-forward
```

In another terminal:

```bash
curl http://127.0.0.1:8801/metrics
make bench-quick
```

## Apply Changes

After rebuilding images or editing manifests:

```bash
make docker-build
make kube-apply
```

Changing ConfigMap data does not hot-reload the Go process. Restart the Server
Deployment after applying configuration changes:

```bash
kubectl rollout restart deployment/server -n mini-llm
kubectl rollout status deployment/server -n mini-llm
```

## Observe And Debug

```bash
kubectl logs deployment/server -n mini-llm
kubectl logs deployment/executor -n mini-llm
kubectl describe pod -n mini-llm -l app=server
kubectl describe pod -n mini-llm -l app=executor
```

The readiness probes determine whether Pod IPs remain in EndpointSlices.
Liveness probe failures cause kubelet to restart the affected container.

## Stop

Delete the entire kind cluster:

```bash
make kube-down
```

This removes the nodes and every Kubernetes resource inside the cluster.
