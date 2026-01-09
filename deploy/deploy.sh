#!/usr/bin/env bash
# Run on the billfold.ddns.net host, from the repo root, after .env is populated.
set -euo pipefail

if [ ! -f .env ]; then
  echo "Missing .env — copy .env.example and fill in production values first." >&2
  exit 1
fi

echo "Pulling latest code..."
git pull --ff-only

echo "Building and starting containers..."
docker compose -f docker-compose.prod.yml up -d --build

echo "Waiting for API health check..."
for i in $(seq 1 30); do
  if curl -fs http://127.0.0.1:8080/ready >/dev/null; then
    echo "API is ready."
    exit 0
  fi
  sleep 2
done

echo "API did not become ready in time." >&2
docker compose -f docker-compose.prod.yml logs --tail=50 api
exit 1
