#!/bin/bash

# ==============================================================================
# Enterprise Deployment Script for Telegram Serverless Platform
#
# Location: /cmd/deploy_cluster.sh
# Usage: From the project root, run: ./cmd/deploy_cluster.sh
# ==============================================================================

# --- Shell Configuration ---
set -e
set -u
set -o pipefail

# --- Dynamic Path Configuration ---
# Get the directory of the script itself.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
# Navigate to the project root (one level up from the script's location).
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- Configuration (Paths are relative to PROJECT_ROOT) ---
readonly NAMESPACE="telegram-serverless"
readonly GO_MODULES=(
    "telegram_serverless/manager"
    "telegram_serverless/tg_gateway"
    "telegram_serverless/tg_proxy"
)
readonly INFRA_MANIFESTS=(
    "telegram_serverless/k8s/namespace/namespace.yaml"
    "telegram_serverless/k8s/zookeeper/service.yaml"
    "telegram_serverless/k8s/zookeeper/statefulset.yaml"
    "telegram_serverless/k8s/kafka/service.yaml"
    "telegram_serverless/k8s/kafka/statefulset.yaml"
    "telegram_serverless/k8s/redis/service.yaml"
    "telegram_serverless/k8s/redis/deployment.yaml"
)
readonly APP_MANIFESTS=(
    "telegram_serverless/manager/k8s/deployment.yaml"
    "telegram_serverless/tg_gateway/k8s/deployment.yaml"
)

# --- Logging Functions ---
log_info() {
    echo -e "\n\e[34m[INFO]\e[0m $1"
}

log_success() {
    echo -e "\e[32m[SUCCESS]\e[0m $1"
}

log_error() {
    echo -e "\e[31m[ERROR]\e[0m $1" >&2
    exit 1
}

# --- Helper Functions ---
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# --- Main Functions ---

check_prerequisites() {
    log_info "Checking for prerequisites..."
    local missing_tools=0
    for tool in docker kubectl minikube; do
        if ! command_exists "$tool"; then
            echo "  - \e[31m$tool is not installed.\e[0m"
            missing_tools=1
        else
            echo "  - \e[32m$tool is installed.\e[0m"
        fi
    done

    if [[ $missing_tools -eq 1 ]]; then
        log_error "Please install the missing tools and try again."
    fi

    if ! minikube status &>/dev/null; then
        log_error "Minikube is not running. Please start it with 'minikube start'."
    fi
    log_success "All prerequisites are met."
}

prepare_environment() {
    log_info "Configuring Docker environment to use Minikube's Docker daemon..."
    eval "$(minikube -p minikube docker-env)"
    log_success "Docker environment is set."
}

resolve_go_dependencies() {
    log_info "Resolving Go module dependencies to prevent build failures..."
    for module in "${GO_MODULES[@]}"; do
        if [ -d "$module" ]; then
            echo "  - Tidying module in '$module'..."
            (cd "$module" && go mod tidy)
        else
            log_error "Directory for Go module '$module' not found."
        fi
    done
    log_success "Go dependencies are up to date."
}

build_and_load_images() {
    log_info "Building Docker images and loading them into Minikube..."
    if [ ! -f "build_images.sh" ]; then
        log_error "'build_images.sh' script not found."
    fi
    ./build_images.sh
    log_success "All Docker images have been built successfully."
}

deploy_kubernetes_resources() {
    log_info "Deploying Kubernetes resources..."

    echo "  - Applying infrastructure manifests..."
    for manifest in "${INFRA_MANIFESTS[@]}"; do
        kubectl apply -f "$manifest"
    done

    echo "  - Waiting for infrastructure to become ready..."
    kubectl wait --for=condition=ready pod -l app=zookeeper -n "$NAMESPACE" --timeout=5m
    kubectl wait --for=condition=ready pod -l app=kafka -n "$NAMESPACE" --timeout=5m
    kubectl wait --for=condition=available deployment/redis -n "$NAMESPACE" --timeout=5m
    log_success "Infrastructure is ready."

    echo "  - Applying application manifests..."
    for manifest in "${APP_MANIFESTS[@]}"; do
        kubectl apply -f "$manifest"
    done

    echo "  - Waiting for applications to become ready..."
    kubectl wait --for=condition=available deployment/manager -n "$NAMESPACE" --timeout=5m
    kubectl wait --for=condition=available deployment/tg-gateway -n "$NAMESPACE" --timeout=5m
    log_success "Applications are ready."
}

print_summary() {
    log_info "Deployment summary:"
    echo "--------------------------------------------------"
    kubectl get all -n "$NAMESPACE"
    echo "--------------------------------------------------"
    log_info "To get the URL for tg-gateway, run:"
    echo "  minikube service tg-gateway -n $NAMESPACE --url"
    log_info "To view logs for a service (e.g., manager), run:"
    echo "  kubectl logs -f -l app=manager -n $NAMESPACE"
    echo "--------------------------------------------------"
    log_success "Deployment finished successfully!"
}

# --- Main Execution ---
main() {
    # Change to the project root directory to ensure all paths are correct.
    cd "$PROJECT_ROOT"

    check_prerequisites
    prepare_environment
    resolve_go_dependencies
    build_and_load_images
    deploy_kubernetes_resources
    print_summary
}

main "$@"

