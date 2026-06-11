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

# Check if Docker daemon is running
if ! docker info >/dev/null 2>&1; then
  echo "❌ Error: Docker daemon is not running."
  echo "👉 Please start Docker Desktop or the Docker daemon and try again."
  exit 1
fi

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
# Detect OS and open browser accordingly
open_browser() {
  local target="frontend/index.html"
  if [[ "$OSTYPE" == "darwin"* ]]; then
    open "$target"
  elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    if command -v xdg-open >/dev/null 2>&1; then
      xdg-open "$target"
    elif command -v sensible-browser >/dev/null 2>&1; then
      sensible-browser "$target"
    else
      echo "Please open $target in your browser manually."
    fi
  elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || "$OSTYPE" == "win32" ]]; then
    # Git Bash, MSYS, Cygwin on Windows
    start "$target" 2>/dev/null || cmd.exe /c start "$target"
  elif grep -qE "(Microsoft|microsoft-standard-WSL|WSL)" /proc/version 2>/dev/null; then
    # WSL (Windows Subsystem for Linux)
    if command -v cmd.exe >/dev/null 2>&1; then
      cmd.exe /c start "$target"
    elif command -v powershell.exe >/dev/null 2>&1; then
      powershell.exe -Command "Start-Process '$target'"
    elif command -v xdg-open >/dev/null 2>&1; then
      xdg-open "$target"
    else
      echo "Please open $target in your browser manually."
    fi
  else
    # Fallback to standard command checks
    if command -v open >/dev/null 2>&1; then
      open "$target"
    elif command -v xdg-open >/dev/null 2>&1; then
      xdg-open "$target"
    else
      echo "Please open $target in your browser manually."
    fi
  fi
}

open_browser

echo "🎉 Setup complete! All services are running in the background."
echo "👉 View runtime logs:  docker compose logs -f"
echo "👉 Stop all services:  docker compose down or run ./stop.sh"
