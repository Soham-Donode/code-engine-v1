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

# Check sandbox runtime registration if targeting runsc
TARGET_RUNTIME="${SANDBOX_RUNTIME:-runsc}"
if [ "$TARGET_RUNTIME" = "runsc" ]; then
  if ! docker info --format '{{json .Runtimes}}' 2>/dev/null | grep -i "runsc" >/dev/null 2>&1; then
    echo "⚠️ Error: gVisor runtime ('runsc') is NOT registered with the Docker daemon."
    echo ""
    echo "👉 To install and register gVisor (runsc) on Linux (ARM64 / x86_64):"
    echo "   (set -e"
    echo "    ARCH=\$(uname -m)"
    echo "    URL=https://storage.googleapis.com/gvisor/releases/release/latest/\${ARCH}"
    echo "    wget \${URL}/runsc \${URL}/runsc.sha512"
    echo "    sha512sum -c runsc.sha512"
    echo "    chmod +x runsc && sudo mv runsc /usr/local/bin/"
    echo "    sudo runsc install"
    echo "    sudo systemctl restart docker)"
    echo ""
    echo "👉 For local development without gVisor, run with the default runc fallback:"
    echo "   SANDBOX_RUNTIME=runc ./start.sh"
    echo ""
    exit 1
  fi
fi

echo "🚀 Starting CodeEngine Docker services in background..."
docker compose up -d --build

echo "🧹 Cleaning up intermediate build layers..."
# Removes temporary build-stage images left behind by multi-stage builds
docker image prune -f >/dev/null 2>&1

echo "⏳ Waiting for API gateway to initialize..."
gateway_ready=false
# Wait up to 10 seconds checking if Nginx responds to preflight or basic health requests
for i in {1..10}; do
  status_code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost/submit -X OPTIONS)
  if [[ "$status_code" =~ ^(200|204|405)$ ]]; then
    echo "✓ CodeEngine API Gateway is online!"
    gateway_ready=true
    break
  fi
  sleep 1
done

if [ "$gateway_ready" = false ]; then
  echo "⚠️ Warning: Gateway taking longer than expected to respond. Proceeding anyway..."
fi

echo "🖥️ Opening IDE in default browser..."
# Detect OS and open browser accordingly
open_browser() {
  local target="frontend/index.html"

  # Convert path to absolute path for better OS launcher compatibility
  if [ -f "$target" ]; then
    target="$DIR/$target"
  fi

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