#!/bin/bash

# Get the directory of this script to ensure relative paths work from anywhere
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$DIR"

echo "🛑 Stopping CodeEngine Docker services..."
docker compose down

echo "✅ All services stopped successfully!"
