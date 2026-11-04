SHELL_PATH = /bin/ash
SHELL = $(if $(wildcard $(SHELL_PATH)),/bin/ash,/bin/bash)

GOLANG          := golang:1.27
ALPINE          := alpine:3.24
KIND            := kindest/node:v1.37.0
POSTGRES        := postgres:18.6
GRAFANA         := grafana/grafana:13.2.1
PROMETHEUS      := prom/prometheus:v3.14.0
TEMPO           := grafana/tempo:3.0.3
LOKI            := grafana/loki:3.7.7
ALLOY           := grafana/alloy:v1.19.2

KIND_CLUSTER    := rameez-starter-cluster
NAMESPACE       := sales-system
SALES_APP       := sales
AUTH_APP        := auth
BASE_IMAGE_NAME := localhost/rameezkhan
VERSION         := 0.0.1
SALES_IMAGE     := $(BASE_IMAGE_NAME)/$(SALES_APP):$(VERSION)
METRICS_IMAGE   := $(BASE_IMAGE_NAME)/metrics:$(VERSION)
AUTH_IMAGE      := $(BASE_IMAGE_NAME)/$(AUTH_APP):$(VERSION)


run: 
	go run api/services/sales/main.go | go run api/tooling/logfmt/main.go

help:
	go run api/services/sales/main.go --help

version:
	go run api/services/sales/main.go --version

dev-up:
	kind create cluster \
		--image $(KIND) \
		--name $(KIND_CLUSTER) \
		--config zarf/k8s/dev/kind-config.yaml

	kubectl wait --timeout=120s --namespace=local-path-storage --for=condition=Available deployment/local-path-provisioner

dev-down:
	kind delete cluster --name $(KIND_CLUSTER)

dev-status:
	watch -n 2 kubectl get pods -o wide --all-namespaces

build: sales 

dev-load:
	kind load docker-image $(SALES_IMAGE) --name $(KIND_CLUSTER)

dev-apply:
	kustomize build zarf/k8s/dev/sales | kubectl apply -f -
	kubectl wait pods --namespace=$(NAMESPACE) --selector app=$(SALES_APP) --timeout=120s --for=condition=Ready

dev-restart:
	kubectl rollout restart deployment $(SALES_APP) --namespace=$(NAMESPACE)

dev-update: build dev-load dev-restart

dev-update-apply: build dev-load dev-apply

dev-logs:
	kubectl logs --namespace=$(NAMESPACE) -l app=$(SALES_APP) --all-containers=true -f --tail=100 --max-log-requests=6 | go run api/tooling/logfmt/main.go -service=$(SALES_APP)


dev-describe-deployment:
	kubectl describe deployment --namespace=$(NAMESPACE) $(SALES_APP)

dev-describe-sales:
	kubectl describe pod --namespace=$(NAMESPACE) -l app=$(SALES_APP)

curl-tester:
	curl -il -X GET http://localhost:3000/test

curl-live:
	curl -il -X GET http://localhost:3000/liveness
	
curl-ready:
	curl -il -X GET http://localhost:3000/readiness



sales:
	docker build \
		-f zarf/docker/dockerfile.sales \
		-t $(SALES_IMAGE) \
		--build-arg BUILD_REF=$(VERSION) \
		--build-arg BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
		--build-arg BUILD_TAG=$(VERSION) \
		.

deps-reset:
	git checkout -- go.mod
	go mod tidy
	go mod vendor

tidy:
	go mod tidy
	go mod vendor