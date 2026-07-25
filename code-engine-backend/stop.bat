@echo off
SETLOCAL EnableDelayedExpansion

:: Get the directory of this script to ensure relative paths work from anywhere
SET "DIR=%~dp0"
CD /D "%DIR%"

:: Check if docker command is available
where docker >nul 2>nul
if %errorlevel% neq 0 (
  echo ❌ Error: 'docker' command is not installed or not in your PATH.
  echo 👉 Please install Docker Desktop for Windows: https://www.docker.com/products/docker-desktop
  pause
  exit /b 1
)

echo 🛑 Stopping CodeEngine Docker services...
docker compose down
if %errorlevel% neq 0 (
  echo ❌ Error: docker compose down failed.
  pause
  exit /b 1
)

echo ✅ All services stopped successfully!
