#!/usr/bin/env bash
# Start a local MariaDB for integration tests WITHOUT Docker.
#
# Integration tests skip when no DB answers, and the skip message says "run
# `make docker-up`" — a dead end without a Docker daemon. mariadbd is
# installed regardless (/usr/sbin/mariadbd) and runs directly against a
# scratch datadir; it refuses to start as root without --user=root, and a
# long scratch path can silently exceed the ~107-char Unix socket path
# limit — both handled below.
#
# USAGE
#   tools/start-test-db.sh          # start (idempotent); prints the DSN to export
#   tools/start-test-db.sh --stop   # stop it, leave the datadir
#   tools/start-test-db.sh --clean  # stop it, delete the datadir
#
# Tests discover it via CHRONICLE_TEST_DB_DSN, printed here. Listens on
# 13306, not 3306, so it never collides with or is mistaken for a real dev
# server.

set -euo pipefail

PORT="${CHRONICLE_TEST_DB_PORT:-13306}"
DATADIR="${CHRONICLE_TEST_DB_DATADIR:-/tmp/chronicle-testdb/data}"
# Short by construction: a socket path over ~107 chars fails with a confusing
# truncated-path error rather than a length error.
SOCKET="${CHRONICLE_TEST_DB_SOCKET:-/tmp/chronicle-testdb.sock}"
PIDFILE="/tmp/chronicle-testdb.pid"
ERRLOG="/tmp/chronicle-testdb.err"

die() { echo "error: $*" >&2; exit 1; }

running() { [ -S "${SOCKET}" ] && mysql --socket="${SOCKET}" -uroot -e "SELECT 1" >/dev/null 2>&1; }

stop_server() {
  if [ -f "${PIDFILE}" ]; then
    local pid; pid="$(cat "${PIDFILE}" 2>/dev/null || true)"
    [ -n "${pid}" ] && kill "${pid}" 2>/dev/null || true
    for _ in $(seq 1 20); do running || break; sleep 0.5; done
  fi
  pkill -f "mariadbd .*${DATADIR}" 2>/dev/null || true
  rm -f "${PIDFILE}"
  echo "stopped"
}

case "${1:-start}" in
  --stop) stop_server; exit 0 ;;
  --clean) stop_server; rm -rf "${DATADIR}"; echo "datadir removed: ${DATADIR}"; exit 0 ;;
  start|"") ;;
  *) die "unknown argument: $1 (expected --stop, --clean, or nothing)" ;;
esac

command -v mariadbd >/dev/null 2>&1 || command -v /usr/sbin/mariadbd >/dev/null 2>&1 \
  || die "mariadbd not installed — this script is for images that ship the server binary without a Docker daemon"
MARIADBD="$(command -v mariadbd 2>/dev/null || echo /usr/sbin/mariadbd)"

if running; then
  echo "already running on socket ${SOCKET} (port ${PORT})"
else
  if [ ! -d "${DATADIR}/mysql" ]; then
    echo "initializing datadir at ${DATADIR} ..."
    mkdir -p "${DATADIR}"
    mysql_install_db --datadir="${DATADIR}" --auth-root-authentication-method=normal >/dev/null 2>&1 \
      || die "mysql_install_db failed"
  fi

  # --user=root is REQUIRED when running as root; without it mariadbd aborts with
  # "Please consult the Knowledge Base to find out how to run mysqld as root!",
  # which reads like a permissions problem and is really a missing flag.
  RUN_AS=()
  [ "$(id -u)" -eq 0 ] && RUN_AS=(--user=root)

  # --skip-grant-tables keeps this a zero-credential scratch server. It is bound
  # to loopback on a non-default port and holds nothing but disposable schemas.
  nohup "${MARIADBD}" "${RUN_AS[@]}" \
    --datadir="${DATADIR}" \
    --socket="${SOCKET}" \
    --port="${PORT}" \
    --bind-address=127.0.0.1 \
    --skip-grant-tables \
    --pid-file="${PIDFILE}" \
    >"${ERRLOG}" 2>&1 &

  # The wait budget is 120s, not 20s: a cold InnoDB start can take ~24s, and a
  # timeout that expires before the server is ready fails non-zero even though
  # mariadbd is happily listening. That failure mode is worse than a slow
  # start — every integration test in this repo SKIPS when it cannot reach a
  # database, and a skipped test reports as a passing package, so a false
  # "did not come up" silently converts the whole integration suite into green
  # nothing. The budget is generous on purpose: being slow costs seconds,
  # being wrong costs a suite.
  # NOT `local` — this block runs at top level, not inside a function, and `local`
  # there is a RUNTIME error that `bash -n` does not catch.
  waited=0
  for _ in $(seq 1 240); do
    running && break
    sleep 0.5
    waited=$((waited + 1))
    # Say something at 20s so a genuinely stuck start is distinguishable from a
    # slow one WHILE it is happening, rather than only in the post-mortem.
    [ "${waited}" -eq 40 ] && echo "still waiting for mariadbd (cold InnoDB start can take ~30s) ..." >&2
  done
  if ! running; then
    tail -8 "${ERRLOG}" >&2
    die "server did not come up after $((waited / 2))s — see ${ERRLOG}. If that log ends in
'ready for connections', the server IS up and this probe is what failed: check that
${SOCKET} is writable and that the mysql client can reach it."
  fi
  [ "${waited}" -gt 40 ] && echo "mariadbd ready after $((waited / 2))s" >&2
  echo "started MariaDB $(mysql --socket="${SOCKET}" -uroot -sN -e 'SELECT VERSION()')"
fi

cat <<EOF

  export CHRONICLE_TEST_DB_DSN='root@tcp(127.0.0.1:${PORT})/'

Then: go test ./... -count=1        (integration tests stop skipping)
      make test-int
Stop:  tools/start-test-db.sh --stop
EOF
