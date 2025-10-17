# Telegram Serverless Bot Platform

Платформа для запуска множества Telegram ботов в Kubernetes с автоматическим масштабированием через KEDA.

## Архитектура

```
Telegram → TG Gateway → Kafka (bot_X_incoming) → Bot Worker → Kafka (bot_X_outgoing) → TG Gateway → Telegram
                ↑                                        ↓
                └─────────── Manager (API) ─────────────┘
                   (создание/удаление ботов,
                    управление репликами)
```

### Компоненты

1. **TG Gateway** - единая точка входа/выхода для Telegram API
   - Принимает webhook'и от Telegram
   - Отправляет сообщения в Kafka топики ботов
   - Читает из Kafka команды ботов и вызывает Telegram Bot API

2. **Manager** - оркестратор ботов
   - REST API для регистрации/удаления ботов
   - Создает Kafka топики
   - Создает Kubernetes Deployment + Secret + KEDA ScaledObject
   - Управляет количеством реплик

3. **Bot Worker** - ваш код бота (любой язык)
   - Читает из Kafka топика `bot_{id}_incoming`
   - Обрабатывает логику
   - Отправляет команды в `bot_{id}_outgoing`

4. **Sidecar** (опционально) - контейнер для мониторинга активности
   - Отслеживает активные запросы
   - Предоставляет метрики для Prometheus
   - Координирует graceful shutdown

## Установка

### Требования
- Kubernetes кластер
- kubectl
- Helm (для установки KEDA)

### 1. Установка KEDA

```bash
helm repo add kedacore https://kedacore.github.io/charts
helm install keda kedacore/keda --namespace keda --create-namespace
```

### 2. Установка базовых компонентов

```bash
# Создать namespace и установить Redis, Kafka, Zookeeper
kubectl apply -f telegram_serverless/k8s/namespace/
kubectl apply -f telegram_serverless/k8s/zookeeper/
kubectl wait --for=condition=ready pod -l app=zookeeper -n telegram-serverless --timeout=300s

kubectl apply -f telegram_serverless/k8s/kafka/
kubectl wait --for=condition=ready pod -l app=kafka -n telegram-serverless --timeout=300s

kubectl apply -f telegram_serverless/k8s/redis/
kubectl wait --for=condition=ready pod -l app=redis -n telegram-serverless --timeout=300s
```

**Или используйте Makefile:**
```bash
make deploy-infra
```

### 3. Сборка и деплой TG Gateway

```bash
cd telegram_serverless/tg_gateway

# Собрать Docker образ
docker build -t your-registry/tg-gateway:latest .
docker push your-registry/tg-gateway:latest

# Применить манифест
kubectl apply -f k8s/deployment.yaml

# Получить внешний IP
kubectl get svc tg-gateway -n telegram-serverless
```

### 4. Сборка и деплой Manager

```bash
cd ../manager

# Собрать Docker образ
docker build -t your-registry/manager:latest .
docker push your-registry/manager:latest

# Применить манифест
kubectl apply -f k8s/deployment.yaml
```

## Использование

### Регистрация нового бота

```bash
# Получить порт Manager
kubectl port-forward svc/manager 8080:8080 -n telegram-serverless

# Создать бота
curl -X POST http://localhost:8080/bots \
  -H "Content-Type: application/json" \
  -d '{
    "bot_token": "123456789:ABCdefGHIjklMNOpqrsTUVwxyz",
    "bot_name": "my_awesome_bot",
    "worker_image": "your-registry/my-bot-worker:latest",
    "min_replicas": 0,
    "max_replicas": 10,
    "env_vars": {
      "OPENAI_API_KEY": "sk-...",
      "CUSTOM_VAR": "value"
    }
  }'
```

**Ответ:**
```json
{
  "bot_id": "bot_abc123def456",
  "status": "created",
  "kafka_topics": {
    "incoming": "bot_abc123def456_incoming",
    "outgoing": "bot_abc123def456_outgoing"
  },
  "webhook_url": "http://your-gateway-ip/webhook/123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
}
```

### Просмотр статуса бота

```bash
curl http://localhost:8080/bots/bot_abc123def456
```

**Ответ:**
```json
{
  "bot_id": "bot_abc123def456",
  "bot_name": "my_awesome_bot",
  "status": "running",
  "replicas": {
    "current": 2,
    "min": 0,
    "max": 10
  },
  "kafka_lag": 15,
  "created_at": "2025-10-14T10:00:00Z"
}
```

### Список всех ботов

```bash
curl http://localhost:8080/bots
```

### Обновление лимитов реплик

```bash
curl -X PATCH http://localhost:8080/bots/bot_abc123def456/replicas \
  -H "Content-Type: application/json" \
  -d '{
    "min_replicas": 1,
    "max_replicas": 20
  }'
```

### Удаление бота

```bash
curl -X DELETE http://localhost:8080/bots/bot_abc123def456
```

## Создание Bot Worker

Bot Worker может быть написан на любом языке. Главное - он должен:

1. Читать из Kafka топика `bot_{bot_id}_incoming`
2. Обрабатывать Update'ы от Telegram
3. Отправлять команды в Kafka топик `bot_{bot_id}_outgoing`

### Пример Worker (Python)

```python
from kafka import KafkaConsumer, KafkaProducer
import json
import os

bot_id = os.getenv('BOT_ID')
kafka_brokers = os.getenv('KAFKA_BROKERS', 'localhost:9092')
incoming_topic = os.getenv('KAFKA_INCOMING_TOPIC')
outgoing_topic = os.getenv('KAFKA_OUTGOING_TOPIC')
bot_token = os.getenv('BOT_TOKEN')

consumer = KafkaConsumer(
    incoming_topic,
    bootstrap_servers=kafka_brokers,
    group_id=f'bot_{bot_id}_workers',
    value_deserializer=lambda m: json.loads(m.decode('utf-8'))
)

producer = KafkaProducer(
    bootstrap_servers=kafka_brokers,
    value_serializer=lambda v: json.dumps(v).encode('utf-8')
)

for message in consumer:
    update = message.value['update']

    # Обработка сообщения
    if 'message' in update:
        text = update['message'].get('text', '')
        chat_id = update['message']['chat']['id']

        # Отправка ответа
        response = {
            'bot_token': bot_token,
            'method': 'sendMessage',
            'params': {
                'chat_id': chat_id,
                'text': f'Получил: {text}'
            }
        }

        producer.send(outgoing_topic, value=response)
```

### Формат сообщений

**Входящее (incoming):**
```json
{
  "bot_id": "bot_abc123",
  "update": {
    "update_id": 123456,
    "message": {
      "message_id": 789,
      "from": {...},
      "chat": {...},
      "text": "Hello"
    }
  }
}
```

**Исходящее (outgoing):**
```json
{
  "bot_token": "123456:ABC",
  "method": "sendMessage",
  "params": {
    "chat_id": 12345,
    "text": "Hello back!",
    "reply_markup": {...}
  }
}
```

## Автоскейлинг

KEDA автоматически масштабирует количество реплик бота на основе:
- Длины очереди в Kafka (`lagThreshold: 5`)
- `minReplicaCount` и `maxReplicaCount`

**Scale to Zero:** Если нет сообщений, количество реплик становится 0 (экономия ресурсов).

## Мониторинг

### Prometheus метрики

- TG Gateway: `http://tg-gateway:9090/metrics`
- Manager: `http://manager:9090/metrics`

### Логи

```bash
# TG Gateway
kubectl logs -f deployment/tg-gateway -n telegram-serverless

# Manager
kubectl logs -f deployment/manager -n telegram-serverless

# Bot Worker
kubectl logs -f deployment/bot-{bot_id} -n telegram-serverless
```

## Переменные окружения Bot Worker

Manager автоматически передает следующие переменные в Pod:

- `BOT_ID` - уникальный ID бота
- `BOT_TOKEN` - токен бота (из Secret)
- `KAFKA_BROKERS` - адреса Kafka брокеров
- `KAFKA_INCOMING_TOPIC` - топик для входящих сообщений
- `KAFKA_OUTGOING_TOPIC` - топик для исходящих команд
- `KAFKA_CONSUMER_GROUP` - группа потребителей
- Все пользовательские `env_vars` из запроса

## Production рекомендации

1. **Используйте managed сервисы:**
   - Managed Kafka (AWS MSK, Confluent Cloud)
   - Managed Redis (AWS ElastiCache, Redis Cloud)

2. **HTTPS для TG Gateway:**
   - Настройте Ingress с TLS сертификатом
   - Telegram требует HTTPS для webhook'ов

3. **Resource limits:**
   - Установите правильные requests/limits для CPU и памяти
   - Настройте HPA для TG Gateway и Manager

4. **Мониторинг:**
   - Prometheus + Grafana
   - AlertManager для алертов

5. **Backup:**
   - Регулярно делайте backup Redis

## Troubleshooting

### Бот не получает сообщения

1. Проверьте webhook:
```bash
curl https://api.telegram.org/bot<TOKEN>/getWebhookInfo
```

2. Проверьте логи TG Gateway:
```bash
kubectl logs -f deployment/tg-gateway -n telegram-serverless
```

3. Проверьте Kafka топики:
```bash
kubectl exec -it kafka-0 -n telegram-serverless -- kafka-topics --list --bootstrap-server localhost:9092
```

### Worker не запускается

1. Проверьте Secret:
```bash
kubectl get secret bot-{bot_id}-secrets -n telegram-serverless -o yaml
```

2. Проверьте Deployment:
```bash
kubectl describe deployment bot-{bot_id} -n telegram-serverless
```

3. Проверьте логи:
```bash
kubectl logs deployment/bot-{bot_id} -n telegram-serverless
```

## Лицензия

MIT
