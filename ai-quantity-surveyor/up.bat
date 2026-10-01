@echo off
setlocal
cd /d "%~dp0"

if not exist ".env" (
  echo .env not found. Copy .env.example to .env and set POSTGRES_PASSWORD first.
  exit /b 1
)

docker compose up -d --build
if errorlevel 1 exit /b %errorlevel%

docker compose ps
echo.
echo Web: http://localhost:3001
echo API: http://localhost:8080
