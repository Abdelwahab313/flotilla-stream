SHELL := /bin/bash

CLUSTER  := flotilla
CONTEXT  := kind-$(CLUSTER)
REGISTRY := ghcr.io/abdelwahab313/flotilla-stream
IMAGES   := simulator=simulator \
            ingest-gateway=services/ingest-gateway \
            location-projector=services/location-projector \
            live-gateway=services/live-gateway
OVERLAY  := deploy/overlays/.generated/local
KUBECTL  := kubectl --context $(CONTEXT)
REPO_URL := https://github.com/Abdelwahab313/flotilla-stream.git
INPUTS   := go.mod go.sum cmd internal services simulator deploy infra Makefile

SHORT := $(shell git rev-parse HEAD | cut -c1-7)
DIRTY := $(shell git status --porcelain -- $(INPUTS) | head -1)
ifneq ($(DIRTY),)
DIFFHASH := $(shell { git diff HEAD --binary -- $(INPUTS); git ls-files -o --exclude-standard -z -- $(INPUTS) | xargs -0 shasum -a 256 2>/dev/null; } | shasum -a 256 | cut -c1-8)
TAG := sha-$(SHORT)-dirty-$(DIFFHASH)
else
TAG := sha-$(SHORT)
endif

.PHONY: demo-r0 up-local preflight images kind-up kind-load deploy-local kind-down tag gitops-local gitops-preflight

demo-r0:
	demo/R0.sh

tag:
	@echo $(TAG)

up-local: preflight images kind-up kind-load deploy-local

preflight:
	@for t in docker kind kubectl go git openssl; do command -v $$t >/dev/null || { echo "missing tool: $$t"; exit 1; }; done
	@if [ -n "$$(docker compose -p flotilla ps -q 2>/dev/null)" ]; then echo "the compose stack is running; stop it first (docker compose down): both do not fit in memory"; exit 1; fi
	@mem=$$(docker info --format '{{.MemTotal}}'); if [ $$mem -lt 7000000000 ]; then echo "docker has $$mem bytes of memory; 7 GB or more is needed"; exit 1; fi
	@if ! kind get clusters 2>/dev/null | grep -qx $(CLUSTER); then \
	  for p in 18080 13000; do if nc -z 127.0.0.1 $$p 2>/dev/null; then echo "port $$p is in use"; exit 1; fi; done; fi

images:
	@set -e; for e in $(IMAGES); do name=$${e%%=*}; dir=$${e#*=}; \
	  echo "building $(REGISTRY)/$$name:$(TAG)"; \
	  docker build -q -f deploy/images/Dockerfile --build-arg SERVICE=$$dir -t $(REGISTRY)/$$name:$(TAG) . >/dev/null; \
	done

kind-up:
	@kind get clusters 2>/dev/null | grep -qx $(CLUSTER) || kind create cluster --config infra/kind/cluster.yaml --wait 120s

kind-load:
	@set -e; for e in $(IMAGES); do kind load docker-image --name $(CLUSTER) $(REGISTRY)/$${e%%=*}:$(TAG); done

$(OVERLAY)/kustomization.yaml: FORCE
	@mkdir -p $(OVERLAY)
	@{ echo "apiVersion: kustomize.config.k8s.io/v1beta1"; echo "kind: Kustomization"; \
	   echo "resources:"; echo "  - ../../local"; echo "images:"; \
	   for e in $(IMAGES); do echo "  - name: $(REGISTRY)/$${e%%=*}"; echo "    newTag: $(TAG)"; done; } > $@

deploy-local: $(OVERLAY)/kustomization.yaml
	$(KUBECTL) apply --server-side -k deploy/base/namespaces
	kubectl kustomize --enable-helm deploy/base/strimzi | $(KUBECTL) apply --server-side -f -
	$(KUBECTL) wait crd -l app=strimzi --for=condition=established --timeout=120s
	@$(KUBECTL) get secret -n flotilla ingest-token >/dev/null 2>&1 || \
	  $(KUBECTL) create secret generic ingest-token -n flotilla --from-literal=token=$$(openssl rand -hex 16)
	kubectl kustomize --enable-helm $(OVERLAY) | $(KUBECTL) apply --server-side -f -
	$(KUBECTL) wait kafka/flotilla -n flotilla --for=condition=Ready --timeout=600s
	$(KUBECTL) wait kafkatopic/position-reported -n flotilla --for=condition=Ready --timeout=120s
	$(KUBECTL) rollout status deploy -n observability -l app.kubernetes.io/name --timeout=600s
	$(KUBECTL) rollout status deploy -n flotilla -l app.kubernetes.io/name --timeout=300s

kind-down:
	kind delete cluster --name $(CLUSTER)

gitops-preflight:
	@if [ -n "$(DIRTY)" ]; then echo "uncommitted changes under $(INPUTS); Argo CD syncs a pushed commit only"; exit 1; fi
	@git fetch --quiet origin
	@if [ -z "$$(git branch -r --contains HEAD)" ]; then echo "HEAD $(SHORT) is not on any branch of origin; push it first"; exit 1; fi

gitops-local: gitops-preflight preflight images kind-up kind-load
	kubectl kustomize --enable-helm deploy/bootstrap/argocd | $(KUBECTL) apply --server-side -f -
	$(KUBECTL) rollout status deploy -n argocd -l app.kubernetes.io/part-of=argocd --timeout=600s
	$(KUBECTL) rollout status statefulset -n argocd argocd-application-controller --timeout=600s
	$(KUBECTL) apply --server-side -k deploy/base/namespaces
	@$(KUBECTL) get secret -n flotilla ingest-token >/dev/null 2>&1 || \
	  $(KUBECTL) create secret generic ingest-token -n flotilla --from-literal=token=$$(openssl rand -hex 16)
	@{ echo "apiVersion: argoproj.io/v1alpha1"; echo "kind: Application"; \
	   echo "metadata: {name: flotilla-local, namespace: argocd}"; \
	   echo "spec:"; echo "  project: default"; \
	   echo "  destination: {server: https://kubernetes.default.svc}"; \
	   echo "  source:"; echo "    repoURL: $(REPO_URL)"; \
	   echo "    targetRevision: $$(git rev-parse HEAD)"; echo "    path: deploy/overlays/local"; \
	   echo "    kustomize:"; echo "      images:"; \
	   for e in $(IMAGES); do echo "        - $(REGISTRY)/$${e%%=*}:$(TAG)"; done; \
	   echo "  syncPolicy:"; echo "    automated: {prune: true, selfHeal: true}"; \
	   echo "    syncOptions: [ServerSideApply=true, SkipDryRunOnMissingResource=true]"; \
	} | $(KUBECTL) apply --server-side -f -
	$(KUBECTL) wait application/flotilla-local -n argocd --for=jsonpath='{.status.sync.status}'=Synced --timeout=900s
	$(KUBECTL) wait application/flotilla-local -n argocd --for=jsonpath='{.status.health.status}'=Healthy --timeout=900s
	@$(KUBECTL) get application/flotilla-local -n argocd -o jsonpath='{.status.sync.revision} {.status.sync.status} {.status.health.status}{"\n"}'

FORCE:
