#!/bin/sh
set -e
echo "Worker starting (APP_ENV=${APP_ENV:-unknown})"
while true; do
  echo "[worker] tick $(date -u +%H:%M:%S)"
  sleep 30
done
