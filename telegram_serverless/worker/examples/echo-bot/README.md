# Echo Bot Example

Простой пример бота, который эхом повторяет все полученные сообщения.

## Сборка

```bash
docker build -t your-registry/echo-bot:latest .
docker push your-registry/echo-bot:latest
```

## Регистрация бота

```bash
curl -X POST http://manager:8080/bots \
  -H "Content-Type: application/json" \
  -d '{
    "bot_token": "YOUR_BOT_TOKEN",
    "bot_name": "echo_bot",
    "worker_image": "your-registry/echo-bot:latest",
    "min_replicas": 0,
    "max_replicas": 5
  }'
```

## Тестирование

1. Отправьте любое сообщение боту в Telegram
2. Бот ответит: `Echo: ваше сообщение`

## Расширение

Вы можете добавить:
- Обработку команд (`/start`, `/help`)
- Inline кнопки
- Работу с файлами
- Базу данных
- Внешние API (OpenAI, и т.д.)
