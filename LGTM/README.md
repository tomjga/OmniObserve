## Step-by-Step Guide: Deploying Grafana for Observability

### 1. Deploy Grafana with Helm (Kubernetes)

#### Prerequisites:
- Kubernetes cluster/Node
- Helm installed
- Persistent Volume provisioner (e.g., AWS EBS, Azure Disk, GCE PD)

#### Deployment Steps:

**a. Add Grafana Helm repository**
```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update
sudo helm show values grafana/loki > loki-values.yaml
sudo helm show values grafana/grafana > grafana-values.yaml
sudo helm show values grafana/tempo > tempo-values.yaml
```

**b. Create values file (`grafana-values.yaml`):**
Refer to [grafana-values.yaml](grafana-values.yaml)

**c. Install Grafana:**
```bash
kubectl create namespace monitoring
# Loki (only Loki + Promtail)
sudo helm install loki grafana/loki -n monitoring -f loki-values.yaml --set grafana.enabled=false  # prevent grafana duplication

# Tempo (single-node mode)
sudo helm install tempo grafana/tempo -n monitoring -f tempo-values.yaml

# Grafana (main dashboard UI)
sudo helm install grafana grafana/grafana -n monitoring -f grafana-values.yaml

# Prometheus (core metrics only)
sudo helm install prometheus prometheus-community/prometheus -n monitoring -f prometheus-values.yaml

sudo helm upgrade --install grafana grafana/grafana -n monitoring -f grafana-values.yaml
sudo helm upgrade --install prometheus prometheus-community/prometheus -n monitoring -f prometheus-values.yaml
sudo helm upgrade --install loki grafana/loki-stack -n monitoring -f loki-values.yaml --set grafana.enabled=false
sudo helm upgrade --install tempo grafana/tempo -n monitoring -f tempo-values.yaml

```

**d. Access Services :**
```bash
kubectl -n monitoring port-forward svc/grafana 3000:80 &                                                                                    
kubectl -n monitoring port-forward svc/prometheus-server 9090:80 &
kubectl -n monitoring port-forward svc/loki 3100:3100 &
kubectl -n monitoring port-forward svc/tempo 3200:3200 &
# Kill after forwarding
kill %1 %2 %3 %4
```
Open `http://localhost:3000` (admin/admin)

### 2. Configure Data Sources in Grafana

After deployment, add these data sources:

#### a. Prometheus Data Source
- Name: `Prometheus`
- URL: `http://prometheus-server.monitoring.svc.cluster.local`
- Type: Prometheus

#### b. Loki Data Source
- Name: `Loki`
- URL: `http://loki.monitoring.svc.cluster.local:3100`
- Type: Loki

#### c. Tempo Data Source
- Name: `Tempo`
- URL: `http://tempo.monitoring.svc.cluster.local:3200`
- Type: Tempo

### 3. Create Sample Dashboards

#### Dashboard 1: Application Metrics (Prometheus)

**Import JSON:**
```json
{
  "title": "KPI Service Metrics",
  "panels": [
    {
      "type": "graph",
      "title": "Request Rate",
      "targets": [{
        "expr": "sum(rate(http_requests_total[5m])) by (endpoint)",
        "legendFormat": "{{endpoint}}"
      }]
    },
    {
      "type": "graph",
      "title": "Error Rate",
      "targets": [{
        "expr": "sum(rate(http_requests_total{code=~\"5..\"}[5m])) / sum(rate(http_requests_total[5m]))",
        "legendFormat": "Error Rate"
      }]
    }
  ]
}
```

#### Dashboard 2: Log Analysis (Loki)

**LogQL Query:**
```sql
{app="kpi-service"} | json | level="error"
```

#### Dashboard 3: Trace Analysis (Tempo)

Configure Trace to Logs:
1. In Tempo data source settings
2. Add Loki as "Trace to logs" data source
3. Map trace ID to `traceID` field in logs

### 4. Configure Application for Observability

Add to your Go application:

**a. Structured Logging with Trace IDs:**
```go
import (
	"go.uber.org/zap"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	
	router.Use(func(c *gin.Context) {w
		span := trace.SpanFromContext(c.Request.Context())
		logger.Info("Request",
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.String("traceID", span.SpanContext().TraceID().String()),
		)
		c.Next()
	})
}
```

**b. OpenTelemetry Tracing:**
```go
import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initTracer() func(context.Context) error {
	exporter, _ := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithEndpoint("tempo.monitoring.svc.cluster.local:4317"),
		otlptracegrpc.WithInsecure(),
	)
	
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp.Shutdown
}

func main() {
	cleanup := initTracer()
	defer cleanup(context.Background())
	
	// Add to Gin middleware
	router.Use(func(c *gin.Context) {
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := otel.Tracer("kpi-service").Start(ctx, c.Request.URL.Path)
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
}
```

### 5. Deploy Supporting Services

**a. Prometheus Helm Chart:**
```bash
helm install prometheus prometheus-community/kube-prometheus-stack -n monitoring
```

**b. Loki Helm Chart:**
```bash
helm install loki grafana/loki-stack -n monitoring
```

**c. Tempo Helm Chart:**
```bash
helm install tempo grafana/tempo -n monitoring
```

### 6. Verify Data Flow

**a. Metrics Flow:**
1. App → Prometheus (scrapes /metrics)
2. Prometheus → Grafana

**b. Logs Flow:**
1. App → stdout
2. Loki (via Promtail) → Grafana

**c. Traces Flow:**
1. App → Tempo
2. Tempo → Grafana

### 7. Sample Grafana Dashboard Queries

**a. Correlate Logs with Traces:**
```sql
{app="kpi-service"} | json | traceID="$__trace.traceId"
```

**b. Error Rate by Endpoint:**
```promql
sum(rate(http_requests_total{code=~"5.."}[5m]) by (endpoint)
/
sum(rate(http_requests_total[5m])) by (endpoint)
```

**c. P99 Latency:**
```promql
histogram_quantile(0.99, 
  sum(rate(http_request_duration_seconds_bucket[5m])) by (le, endpoint)
```

### 8. Production Recommendations

1. **Security:**
   - Enable TLS for all connections
   - Use secrets for credentials
   - Set resource quotas

2. **Scalability:**
   ```yaml
   # tempo-values.yaml
   distributor:
     replicas: 3
   ingester:
     replicas: 3
   ```

3. **Storage:**
   - Use S3/GCS for Loki and Tempo long-term storage
   - Configure Prometheus Thanos for long-term metrics

4. **Alerting:**
   ```yaml
   # In Prometheus values
   alertmanager:
     enabled: true
   ```

### Next Steps:

1. Deploy Prometheus stack for metrics collection
2. Set up Loki for log aggregation
3. Configure Tempo for distributed tracing
4. Create comprehensive dashboards in Grafana
5. Set up alerts based on SLOs

This setup provides a complete observability pipeline where you can:
- View real-time metrics in Grafana
- Search and analyze logs in Loki
- Trace requests across services with Tempo
- Correlate metrics, logs, and traces for full observability