# Telegram Serverless Bot Platform

Платформа для запуска множества Telegram ботов в Kubernetes с автоматическим масштабированием через KEDA.

## Развертывание

### 1. Установка NGINX Ingress Controller (если еще не установлен)
```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx --namespace ingress-nginx --create-namespace
```

### 2. Развертывание платформы
```bash
helm upgrade --install tg-bot-serverless ./tg-bot-serverless -f ./tg-bot-serverless/values.yaml --namespace telegram-serverless --create-namespace
```
