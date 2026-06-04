# LGTM secrets

Credentials for the stack live in Kubernetes Secrets, **never** in the committed
Helm values. This directory holds `*.example` templates; the real Secrets are
gitignored (`*-secret.yaml`).

## Create the secrets

```bash
kubectl create namespace monitoring   # if it doesn't exist

# Grafana admin login (referenced by grafana-values.yaml -> admin.existingSecret)
cp grafana-admin-secret.yaml.example grafana-admin-secret.yaml
#   edit admin-password, then:
kubectl -n monitoring apply -f grafana-admin-secret.yaml

# Loki S3/MinIO credentials (referenced by loki-values.yaml -> extraEnvFrom)
cp loki-s3-secret.yaml.example loki-s3-secret.yaml
#   set AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY, then:
kubectl -n monitoring apply -f loki-s3-secret.yaml
```

The `.yaml` copies match the gitignore rule `*-secret.yaml`, so they cannot be
committed by accident. CI (Trivy secret scan) is a second line of defence.

> For local MinIO, the access/secret keys are the ones you generate in the MinIO
> console (or the root user from `../minio/.env`).
