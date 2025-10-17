# Echo Bot with Sidecar Integration

Echo bot с интеграцией sidecar для отслеживания активных запросов и graceful shutdown.

## Как это работает

1. **При получении сообщения:**
   - Worker вызывает `POST /start-request` на sidecar
   - Sidecar инкрементирует счетчик активных запросов

2. **После обработки:**
   - Worker вызывает `POST /end-request?duration=1.2s` на sidecar
   - Sidecar декрементирует счетчик и записывает метрики

3. **При graceful shutdown:**
   - Kubernetes отправляет SIGTERM
   - Sidecar ждет завершения всех активных запросов (до 30 секунд)
   - Только после этого pod удаляется

## Преимущества

- ✅ **Graceful shutdown** - не теряем сообщения при масштабировании вниз
- ✅ **Метрики** - отслеживание активных запросов, времени обработки
- ✅ **Мониторинг** - видим состояние бота в реальном времени
- ✅ **KEDA масштабирует только по Kafka lag** - просто и надежно

## Сборка

```bash
docker build -t your-registry/echo-bot-sidecar:latest .
docker push your-registry/echo-bot-sidecar:latest
```

## Регистрация

```bash
curl -X POST http://manager:8080/bots \
  -H "Content-Type: application/json" \
  -d '{
    "bot_token": "YOUR_BOT_TOKEN",
    "bot_name": "echo_bot_sidecar",
    "worker_image": "your-registry/echo-bot-sidecar:latest",
    "min_replicas": 0,
    "max_replicas": 5
  }'
```

## Мониторинг

### Проверить метрики sidecar
```bash
kubectl port-forward pod/bot-{bot_id}-xxx 9091:9091 -n telegram-serverless
curl http://localhost:9091/metrics
```

### Проверить активность
```bash
kubectl port-forward pod/bot-{bot_id}-xxx 8081:8081 -n telegram-serverless
curl http://localhost:8081/metrics
```

Ответ:
```json
{
  "active_requests": 2,
  "total_requests": 150,
  "last_request_time": "2025-10-14T12:30:45Z",
  "start_time": "2025-10-14T10:00:00Z",
  "is_processing": true,
  "idle_duration_seconds": 0
}
```

## Sidecar Endpoints

| Endpoint | Method | Описание |
|----------|--------|----------|
| `/start-request` | POST | Начало обработки запроса |
| `/end-request?duration=1s` | POST | Конец обработки запроса |
| `/metrics` | GET | Текущие метрики активности |
| `/health` | GET | Health check |
| `/ready` | GET | Readiness check |
| `/metrics` (9091) | GET | Prometheus metrics |

## Интеграция в свой бот

### Python

```python
import requests
import time

SIDECAR_URL = os.getenv('SIDECAR_URL', 'http://localhost:8081')

def process_message(update):
    start_time = time.time()

    # Notify start
    try:
        requests.post(f"{SIDECAR_URL}/start-request", timeout=1)
    except:
        pass

    try:
        # Your bot logic here
        handle_update(update)
    finally:
        # Notify end
        duration = time.time() - start_time
        try:
            requests.post(
                f"{SIDECAR_URL}/end-request?duration={duration}s",
                timeout=1
            )
        except:
            pass
```

### Go

```go
package main

import (
    "fmt"
    "net/http"
    "os"
    "time"
)

var sidecarURL = os.Getenv("SIDECAR_URL")

func startRequest() {
    http.Post(sidecarURL+"/start-request", "", nil)
}

func endRequest(duration time.Duration) {
    url := fmt.Sprintf("%s/end-request?duration=%s", sidecarURL, duration)
    http.Post(url, "", nil)
}

func processMessage(update Update) {
    start := time.Now()
    startRequest()
    defer endRequest(time.Since(start))

    // Your bot logic
    handleUpdate(update)
}
```

### Node.js

```javascript
const axios = require('axios');
const SIDECAR_URL = process.env.SIDECAR_URL || 'http://localhost:8081';

async function processMessage(update) {
  const startTime = Date.now();

  try {
    await axios.post(`${SIDECAR_URL}/start-request`);
  } catch (err) {
    // Ignore errors
  }

  try {
    // Your bot logic
    await handleUpdate(update);
  } finally {
    const duration = (Date.now() - startTime) / 1000;
    try {
      await axios.post(`${SIDECAR_URL}/end-request?duration=${duration}s`);
    } catch (err) {
      // Ignore errors
    }
  }
}
```

## Troubleshooting

### Sidecar не отвечает

```bash
# Проверить что sidecar запущен
kubectl get pods -n telegram-serverless
kubectl logs bot-{bot_id}-xxx -c sidecar -n telegram-serverless

# Проверить что порты открыты
kubectl exec bot-{bot_id}-xxx -c bot -n telegram-serverless -- curl localhost:8081/health
```

### Pod не завершается при scale down

Проверьте логи sidecar - возможно есть зависшие запросы:
```bash
kubectl logs bot-{bot_id}-xxx -c sidecar -n telegram-serverless
```

Если запросы зависли, увеличьте `GRACE_PERIOD_SECONDS` в deployment.
