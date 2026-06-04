#!/usr/bin/env bash
set -euo pipefail
# Load generator for the auto-rollback demo. Point at the service via a port-forward:
#   kubectl -n monitoring port-forward svc/api-service 8080:8080
#
#   ./demo/load.sh            steady healthy traffic (200s)
#   ./demo/load.sh --errors   flood 5xx to breach the error-rate SLO (500s)

URL="${SERVICE_URL:-http://localhost:8080}"

if [ "${1:-}" = "--errors" ]; then
  path="/kpi/errors?error_rate=100"
  echo "flooding 5xx -> ${URL}${path}  (Ctrl-C to stop)"
else
  path="/kpi/availability?success_rate=100"
  echo "steady healthy load -> ${URL}${path}  (Ctrl-C to stop)"
fi

while true; do
  curl -s -o /dev/null "${URL}${path}" || true
  sleep 0.1
done
