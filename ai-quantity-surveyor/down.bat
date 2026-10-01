@echo off
setlocal
cd /d "%~dp0"

rem Data in the PostgreSQL volume is kept; use "docker compose down -v" only to delete it.
docker compose down
