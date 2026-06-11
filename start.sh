#!/bin/bash

# Get the directory of this script to ensure relative paths work from anywhere
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$DIR"

echo "🚀 Starting CodeEngine Docker services in background..."
docker compose up -d --build

echo "⏳ Waiting for API gateway to initialize..."
# Wait up to 10 seconds checking if Nginx responds to preflight submit requests
for i in {1..10}; do
  if curl -s -o /dev/null -w "%{http_code}" http://localhost/submit -X OPTIONS | grep -q "204"; then
    echo "✓ CodeEngine API Gateway is online!"
    break
  fi
  sleep 1
done

echo "🖥️ Opening IDE in default browser..."
open frontend/index.html

echo "🎉 Setup complete! All services are running in the background."
echo "👉 View runtime logs:  docker compose logs -f"
echo "👉 Stop all services:  docker compose down"
