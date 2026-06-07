#!/bin/sh
set -e

DATA_DIR="${WEBSSH_DATA_DIR:-/app/data}"
APP_USER="webssh"
APP_UID="1000"

echo "[entrypoint] data dir: $DATA_DIR"

# 1Panel / docker-compose 挂载的宿主机目录属主通常是 root，
# 容器内 webssh (uid 1000) 写不进去。尝试 chown，失败也不致命。
if [ -d "$DATA_DIR" ]; then
  chown -R "$APP_UID:$APP_UID" "$DATA_DIR" 2>/dev/null || \
    echo "[entrypoint] warn: chown $DATA_DIR failed (容器内可能没权限)，如持续写入失败请在宿主机执行: chown -R 1000:1000 $DATA_DIR"
else
  mkdir -p "$DATA_DIR"
  chown -R "$APP_UID:$APP_UID" "$DATA_DIR" 2>/dev/null || true
fi

# 切换到 webssh 用户执行
if [ "$(id -u)" = "0" ]; then
  exec su-exec "$APP_UID:$APP_UID" "$@"
else
  exec "$@"
fi
