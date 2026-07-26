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

:: Check if Docker daemon is running
docker info >nul 2>nul
if %errorlevel% neq 0 (
  echo ❌ Error: Docker daemon is not running.
  echo 👉 Please start Docker Desktop and try again.
  pause
  exit /b 1
)

echo 🚀 Starting CodeEngine Docker services in background...
docker compose up -d --build
if %errorlevel% neq 0 (
  echo ❌ Error: docker compose up failed.
  pause
  exit /b 1
)

:: Check if curl is available for health checks
where curl >nul 2>nul
if %errorlevel% neq 0 (
  echo ⏳ Waiting for API gateway to initialize (curl not found, skipping health check)...
  timeout /t 3 >nul 2>&1
  goto :gateway_online
)

echo ⏳ Waiting for API gateway to initialize...
:: Wait up to 10 seconds checking if Nginx responds to preflight submit requests
FOR /L %%i IN (1,1,10) DO (
  curl -s -o NUL -w "%%{http_code}" http://localhost/submit -X OPTIONS | findstr "204" >nul
  IF !ERRORLEVEL! EQU 0 (
    echo ✓ CodeEngine API Gateway is online!
    goto :gateway_online
  )
  timeout /t 1 >nul 2>&1
)
echo ⚠️ Warning: API Gateway is taking longer than expected to start.

:gateway_online
echo 🖥️ Opening IDE in default browser...
start frontend/index.html

echo 🎉 Setup complete! All services are running in the background.
echo 👉 View runtime logs:  docker compose logs -f
echo 👉 Stop all services:  docker compose down or run stop.bat
