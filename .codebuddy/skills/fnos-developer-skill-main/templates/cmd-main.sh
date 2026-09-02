#!/bin/bash
set -u

APP_BIN="${TRIM_APPDEST}/server/myapp"
PID_FILE="${TRIM_PKGVAR}/myapp.pid"
LOG_FILE="${TRIM_PKGVAR}/myapp.log"
SOCKET_FILE="${TRIM_APPDEST}/app.sock"

is_running() {
  [ -f "$PID_FILE" ] || return 1
  pid="$(cat "$PID_FILE" 2>/dev/null)"
  [ -n "$pid" ] || return 1
  kill -0 "$pid" 2>/dev/null
}

start_app() {
  if is_running; then
    return 0
  fi

  if [ ! -x "$APP_BIN" ]; then
    echo "Application executable is missing or not executable." > "$TRIM_TEMP_LOGFILE"
    return 1
  fi

  mkdir -p "$TRIM_PKGVAR"
  rm -f "$PID_FILE" "$SOCKET_FILE"

  # This template assumes config/privilege uses run-as=package.
  # Add only application-specific, validated arguments here.
  nohup "$APP_BIN" \
    --socket "$SOCKET_FILE" \
    --config "$TRIM_PKGETC/config.json" \
    >>"$LOG_FILE" 2>&1 &
  pid=$!
  echo "$pid" > "$PID_FILE"

  sleep 1
  if ! kill -0 "$pid" 2>/dev/null; then
    rm -f "$PID_FILE" "$SOCKET_FILE"
    echo "Application service failed to start. Check the application log." > "$TRIM_TEMP_LOGFILE"
    return 1
  fi
}

stop_app() {
  if ! is_running; then
    rm -f "$PID_FILE" "$SOCKET_FILE"
    return 0
  fi

  pid="$(cat "$PID_FILE")"
  kill "$pid" 2>/dev/null || true

  count=0
  while kill -0 "$pid" 2>/dev/null && [ "$count" -lt 20 ]; do
    sleep 1
    count=$((count + 1))
  done

  if kill -0 "$pid" 2>/dev/null; then
    kill -KILL "$pid" 2>/dev/null || true
  fi

  rm -f "$PID_FILE" "$SOCKET_FILE"
}

case "${1:-}" in
  start)
    start_app
    ;;
  stop)
    stop_app
    ;;
  status)
    if is_running; then
      exit 0
    fi
    exit 3
    ;;
  *)
    exit 1
    ;;
esac
