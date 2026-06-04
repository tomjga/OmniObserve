#!/usr/bin/env bash
# Credentials are read from the environment so they are never committed.
#   export ACCESS_KEY=... SECRET_KEY=...   (or: set -a; source ../minio/.env; set +a)
ACCESS_KEY="${ACCESS_KEY:?set ACCESS_KEY in the environment}"
SECRET_KEY="${SECRET_KEY:?set SECRET_KEY in the environment}"
BUCKET="loki-chunks"

curl -X GET "http://localhost:9000/$BUCKET?max-keys=1" \
  -H "Host: localhost:9000" \
  -"
  -H "x-amz-date: $(date -u +'%Y%m%dT%H%M%SZ')" \
  -H "Authorization: AWS4-HMAC-SHA256 Credential=$ACCESS_KEY/$(date -u +'%Y%m%d')/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=$( \
    printf "AWS4-HMAC-SHA256\n%s\n%s/us-east-1/s3/aws4_request\n%s" \
    "$(date -u +'%Y%m%dT%H%M%SZ')" \
    "$(date -u +'%Y%m%d')" \
    "$(echo -en "GET\n/$BUCKET/\nmax-keys=1\nhost:localhost:9000\nx-amz-date:$(date -u +'%Y%m%dT%H%M%SZ')\n\nhost;x-amz-date\n$(echo -n | sha256sum | cut -d' ' -f1)" | \
    sha256sum | cut -d' ' -f1)" | \
    openssl dgst -sha256 -mac HMAC -macopt "key:AWS4$SECRET_KEY" -macopt hexkey: -binary | \
    openssl dgst -sha256 -mac HMAC -macopt hexkey:"$(echo -n "us-east-1" | openssl dgst -sha256 -mac HMAC -macopt "key:AWS4$SECRET_KEY" -binary | xxd -p)" -binary | \
    openssl dgst -sha256 -mac HMAC -macopt hexkey:"$(echo -n "s3" | openssl dgst -sha256 -mac HMAC -macopt "key:$(echo -n "us-east-1" | openssl dgst -sha256 -mac HMAC -macopt "key:AWS4$SECRET_KEY" -binary | xxd -p)" -binary | xxd -p)" -binary | \
    xxd -p
  )"