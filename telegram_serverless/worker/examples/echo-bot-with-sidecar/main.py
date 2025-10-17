#!/usr/bin/env python3
"""
Echo Bot Worker with Sidecar Integration

Reads from Kafka incoming topic, echoes messages back via outgoing topic.
Integrates with sidecar for activity tracking and graceful shutdown.
"""

import json
import os
import signal
import sys
import time
import requests
from kafka import KafkaConsumer, KafkaProducer
from kafka.errors import KafkaError

# Configuration from environment variables
BOT_ID = os.getenv('BOT_ID')
BOT_TOKEN = os.getenv('BOT_TOKEN')
KAFKA_BROKERS = os.getenv('KAFKA_BROKERS', 'localhost:9092').split(',')
INCOMING_TOPIC = os.getenv('KAFKA_INCOMING_TOPIC')
OUTGOING_TOPIC = os.getenv('KAFKA_OUTGOING_TOPIC')
CONSUMER_GROUP = os.getenv('KAFKA_CONSUMER_GROUP')
SIDECAR_URL = os.getenv('SIDECAR_URL', 'http://localhost:8081')

# Validate required env vars
required_vars = ['BOT_ID', 'BOT_TOKEN', 'KAFKA_INCOMING_TOPIC', 'KAFKA_OUTGOING_TOPIC', 'CONSUMER_GROUP']
for var in required_vars:
    if not os.getenv(var):
        print(f"ERROR: {var} environment variable is required", file=sys.stderr)
        sys.exit(1)

print(f"Starting echo bot worker for {BOT_ID}")
print(f"Kafka brokers: {KAFKA_BROKERS}")
print(f"Incoming topic: {INCOMING_TOPIC}")
print(f"Outgoing topic: {OUTGOING_TOPIC}")
print(f"Consumer group: {CONSUMER_GROUP}")
print(f"Sidecar URL: {SIDECAR_URL}")

# Sidecar helper functions
def sidecar_start_request():
    """Notify sidecar that we started processing a request"""
    try:
        requests.post(f"{SIDECAR_URL}/start-request", timeout=1)
    except Exception as e:
        print(f"Warning: failed to notify sidecar start: {e}", file=sys.stderr)

def sidecar_end_request(duration_seconds=0):
    """Notify sidecar that we finished processing a request"""
    try:
        requests.post(
            f"{SIDECAR_URL}/end-request?duration={duration_seconds}s",
            timeout=1
        )
    except Exception as e:
        print(f"Warning: failed to notify sidecar end: {e}", file=sys.stderr)

# Initialize Kafka consumer
consumer = KafkaConsumer(
    INCOMING_TOPIC,
    bootstrap_servers=KAFKA_BROKERS,
    group_id=CONSUMER_GROUP,
    value_deserializer=lambda m: json.loads(m.decode('utf-8')),
    auto_offset_reset='earliest',
    enable_auto_commit=True,
)

# Initialize Kafka producer
producer = KafkaProducer(
    bootstrap_servers=KAFKA_BROKERS,
    value_serializer=lambda v: json.dumps(v).encode('utf-8'),
    acks='all',
)

# Graceful shutdown handler
def signal_handler(sig, frame):
    print("\nShutting down gracefully...")
    consumer.close()
    producer.close()
    sys.exit(0)

signal.signal(signal.SIGINT, signal_handler)
signal.signal(signal.SIGTERM, signal_handler)

print("Bot worker ready, waiting for messages...")

# Main message processing loop
try:
    for message in consumer:
        start_time = time.time()

        # Notify sidecar that we started processing
        sidecar_start_request()

        try:
            data = message.value
            bot_id = data.get('bot_id')
            update = data.get('update', {})

            print(f"Received update: {update.get('update_id')}")

            # Handle text messages
            if 'message' in update and 'text' in update['message']:
                text = update['message']['text']
                chat_id = update['message']['chat']['id']
                message_id = update['message']['message_id']

                print(f"Processing message from chat {chat_id}: {text}")

                # Send echo response
                response = {
                    'bot_token': BOT_TOKEN,
                    'method': 'sendMessage',
                    'params': {
                        'chat_id': chat_id,
                        'text': f'Echo: {text}',
                        'reply_to_message_id': message_id,
                    }
                }

                producer.send(OUTGOING_TOPIC, value=response)
                producer.flush()
                print(f"Sent response to chat {chat_id}")

            # Handle callback queries
            elif 'callback_query' in update:
                callback_query = update['callback_query']
                callback_id = callback_query['id']
                data_received = callback_query.get('data', '')

                print(f"Processing callback query: {data_received}")

                # Answer callback query
                response = {
                    'bot_token': BOT_TOKEN,
                    'method': 'answerCallbackQuery',
                    'params': {
                        'callback_query_id': callback_id,
                        'text': f'Received: {data_received}',
                    }
                }

                producer.send(OUTGOING_TOPIC, value=response)
                producer.flush()
                print("Answered callback query")

            else:
                print(f"Unhandled update type: {update.keys()}")

        except Exception as e:
            print(f"Error processing message: {e}", file=sys.stderr)
        finally:
            # Always notify sidecar that we're done
            duration = time.time() - start_time
            sidecar_end_request(duration)

except KafkaError as e:
    print(f"Kafka error: {e}", file=sys.stderr)
    sys.exit(1)
except Exception as e:
    print(f"Unexpected error: {e}", file=sys.stderr)
    sys.exit(1)
