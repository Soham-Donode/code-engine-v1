#!/bin/bash

# Get the directory of this script to ensure relative paths work from anywhere
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$DIR"

# Check if docker command is available
if ! command -v docker >/dev/null 2>&1; then
  echo "❌ Error: 'docker' command is not installed or not in your PATH."
  echo "👉 Please install Docker: https://docs.docker.com/get-docker/"
  exit 1
fi

echo "🛑 Stopping CodeEngine Docker services..."
docker compose down

echo "🧹 Cleaning up unused build cache..."
docker image prune -f >/dev/null 2>&1

echo "✅ All services stopped successfully!"
echo "💡 Tip: To also remove database volumes and start completely fresh, run: docker compose down -v"