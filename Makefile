.PHONY: help build-gateway build-manager build-sidecar build-all push-gateway push-manager push-sidecar push-all deploy-infra deploy-gateway deploy-manager deploy-all clean clean-bots logs-gateway logs-manager logs-sidecar port-forward-manager

# Configuration
REGISTRY ?= your-registry
TAG ?= latest
NAMESPACE ?= telegram-serverless

help:
	@echo "Available targets:"
	@echo "  build-gateway      - Build TG Gateway Docker image"
	@echo "  build-manager      - Build Manager Docker image"
	@echo "  build-sidecar      - Build Sidecar Docker image"
	@echo "  build-all          - Build all Docker images"
	@echo "  push-gateway       - Push TG Gateway image to registry"
	@echo "  push-manager       - Push Manager image to registry"
	@echo "  push-sidecar       - Push Sidecar image to registry"
	@echo "  push-all           - Push all images to registry"
	@echo "  deploy-infra       - Deploy infrastructure (Redis, Kafka, KEDA)"
	@echo "  deploy-gateway     - Deploy TG Gateway"
	@echo "  deploy-manager     - Deploy Manager"
	@echo "  deploy-all         - Deploy everything"
	@echo "  clean              - Remove all deployments"
	@echo "  clean-bots         - Remove all bot deployments only"
	@echo "  logs-gateway       - Follow TG Gateway logs"
	@echo "  logs-manager       - Follow Manager logs"
	@echo "  logs-sidecar BOT_ID=<id> - Follow Sidecar logs for specific bot"
	@echo "  port-forward-manager - Port-forward Manager API to localhost:8080"
	@echo ""
	@echo "Environment variables:"
	@echo "  REGISTRY=${REGISTRY}"
	@echo "  TAG=${TAG}"
	@echo "  NAMESPACE=${NAMESPACE}"

# Build targets
build-gateway:
	@echo "Building TG Gateway..."
	docker build -t $(REGISTRY)/tg-gateway:$(TAG) telegram_serverless/tg_gateway/

build-manager:
	@echo "Building Manager..."
	docker build -t $(REGISTRY)/manager:$(TAG) telegram_serverless/manager/

build-sidecar:
	@echo "Building Sidecar..."
	docker build -t $(REGISTRY)/sidecar:$(TAG) telegram_serverless/sidecar/

build-all: build-gateway build-manager build-sidecar

# Push targets
push-gateway:
	@echo "Pushing TG Gateway..."
	docker push $(REGISTRY)/tg-gateway:$(TAG)

push-manager:
	@echo "Pushing Manager..."
	docker push $(REGISTRY)/manager:$(TAG)

push-sidecar:
	@echo "Pushing Sidecar..."
	docker push $(REGISTRY)/sidecar:$(TAG)

push-all: push-gateway push-manager push-sidecar

# Deploy targets
deploy-infra:
	@echo "Deploying infrastructure..."
	@echo "Creating namespace..."
	kubectl apply -f telegram_serverless/k8s/namespace/
	@echo "Deploying Zookeeper..."
	kubectl apply -f telegram_serverless/k8s/zookeeper/
	@echo "Waiting for Zookeeper..."
	kubectl wait --for=condition=ready pod -l app=zookeeper -n $(NAMESPACE) --timeout=300s
	@echo "Deploying Kafka..."
	kubectl apply -f telegram_serverless/k8s/kafka/
	@echo "Waiting for Kafka..."
	kubectl wait --for=condition=ready pod -l app=kafka -n $(NAMESPACE) --timeout=300s
	@echo "Deploying Redis..."
	kubectl apply -f telegram_serverless/k8s/redis/
	@echo "Waiting for Redis..."
	kubectl wait --for=condition=ready pod -l app=redis -n $(NAMESPACE) --timeout=300s
	@echo "Infrastructure deployment complete!"

deploy-gateway:
	@echo "Deploying TG Gateway..."
	kubectl apply -f telegram_serverless/tg_gateway/k8s/deployment.yaml

deploy-manager:
	@echo "Deploying Manager..."
	kubectl apply -f telegram_serverless/manager/k8s/deployment.yaml

deploy-all: deploy-infra deploy-gateway deploy-manager
	@echo "All components deployed!"
	@echo ""
	@echo "Get TG Gateway external IP:"
	@echo "  kubectl get svc tg-gateway -n $(NAMESPACE)"
	@echo ""
	@echo "Port-forward Manager API:"
	@echo "  kubectl port-forward svc/manager 8080:8080 -n $(NAMESPACE)"

# Clean targets
clean:
	@echo "Removing all deployments..."
	-kubectl delete -f telegram_serverless/manager/k8s/deployment.yaml
	-kubectl delete -f telegram_serverless/tg_gateway/k8s/deployment.yaml
	-kubectl delete -f telegram_serverless/k8s/kafka/
	-kubectl delete -f telegram_serverless/k8s/zookeeper/
	-kubectl delete -f telegram_serverless/k8s/redis/
	-kubectl delete -f telegram_serverless/k8s/namespace/
	@echo "Cleanup complete!"

clean-bots:
	@echo "Removing all bot deployments..."
	-kubectl delete deployment -l app=telegram-bot -n $(NAMESPACE)
	-kubectl delete secret -l app=telegram-bot -n $(NAMESPACE)
	-kubectl delete scaledobject -l app=telegram-bot -n $(NAMESPACE)
	@echo "All bots removed!"

# Development helpers
logs-gateway:
	kubectl logs -f deployment/tg-gateway -n $(NAMESPACE)

logs-manager:
	kubectl logs -f deployment/manager -n $(NAMESPACE)

logs-sidecar:
	@if [ -z "$(BOT_ID)" ]; then \
		echo "Error: BOT_ID is required. Usage: make logs-sidecar BOT_ID=bot_abc123"; \
		exit 1; \
	fi
	@POD=$$(kubectl get pods -n $(NAMESPACE) -l bot-id=$(BOT_ID) -o jsonpath='{.items[0].metadata.name}' 2>/dev/null); \
	if [ -z "$$POD" ]; then \
		echo "Error: No pod found for bot $(BOT_ID)"; \
		exit 1; \
	fi; \
	echo "Following logs for pod $$POD (sidecar container)..."; \
	kubectl logs -f $$POD -c sidecar -n $(NAMESPACE)

port-forward-manager:
	kubectl port-forward svc/manager 8080:8080 -n $(NAMESPACE)
