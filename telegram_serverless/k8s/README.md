# Kubernetes Manifests

Infrastructure components for Telegram Serverless Bot Platform.

## Directory Structure

```
k8s/
├── namespace/
│   └── namespace.yaml           # telegram-serverless namespace
├── redis/
│   ├── deployment.yaml          # Redis deployment
│   └── service.yaml             # Redis service
├── zookeeper/
│   ├── statefulset.yaml         # Zookeeper statefulset
│   └── service.yaml             # Zookeeper headless service
└── kafka/
    ├── statefulset.yaml         # Kafka statefulset
    └── service.yaml             # Kafka service
```

## Deployment Order

```bash
# 1. Create namespace
kubectl apply -f namespace/

# 2. Deploy Zookeeper (required for Kafka)
kubectl apply -f zookeeper/

# 3. Wait for Zookeeper to be ready
kubectl wait --for=condition=ready pod -l app=zookeeper -n telegram-serverless --timeout=300s

# 4. Deploy Kafka
kubectl apply -f kafka/

# 5. Wait for Kafka to be ready
kubectl wait --for=condition=ready pod -l app=kafka -n telegram-serverless --timeout=300s

# 6. Deploy Redis
kubectl apply -f redis/

# 7. Wait for Redis to be ready
kubectl wait --for=condition=ready pod -l app=redis -n telegram-serverless --timeout=300s
```

## Deploy All at Once

```bash
kubectl apply -f namespace/ -f zookeeper/ -f kafka/ -f redis/
```

## Verify Deployment

```bash
# Check all pods
kubectl get pods -n telegram-serverless

# Check all services
kubectl get svc -n telegram-serverless

# Check logs
kubectl logs -l app=kafka -n telegram-serverless
kubectl logs -l app=zookeeper -n telegram-serverless
kubectl logs -l app=redis -n telegram-serverless
```

## Testing Components

### Test Redis
```bash
kubectl run -it --rm redis-test --image=redis:7-alpine --restart=Never -n telegram-serverless -- redis-cli -h redis-service ping
```

### Test Kafka
```bash
# Create test topic
kubectl exec -it kafka-0 -n telegram-serverless -- kafka-topics --create --topic test --partitions 3 --replication-factor 1 --bootstrap-server localhost:9092

# List topics
kubectl exec -it kafka-0 -n telegram-serverless -- kafka-topics --list --bootstrap-server localhost:9092

# Delete test topic
kubectl exec -it kafka-0 -n telegram-serverless -- kafka-topics --delete --topic test --bootstrap-server localhost:9092
```

## Production Considerations

⚠️ **These manifests are for development/testing purposes.**

For production, consider:

1. **Persistence**
   - Use PersistentVolumeClaims instead of emptyDir
   - Configure proper storage classes
   - Set up backup/restore strategies

2. **High Availability**
   - Run 3+ Kafka replicas
   - Run 3+ Zookeeper replicas
   - Use Redis Sentinel or Redis Cluster

3. **Resources**
   - Adjust CPU/Memory limits based on load
   - Monitor and tune Kafka/Zookeeper settings
   - Configure JVM settings for Kafka

4. **Security**
   - Enable authentication (SASL for Kafka)
   - Enable encryption (TLS)
   - Use NetworkPolicies
   - Configure RBAC properly

5. **Monitoring**
   - Add Prometheus exporters
   - Configure alerts
   - Set up dashboards

6. **Managed Services**
   - Consider AWS MSK for Kafka
   - Consider AWS ElastiCache for Redis
   - Reduces operational overhead

## Cleanup

```bash
kubectl delete -f kafka/ -f zookeeper/ -f redis/ -f namespace/
```
