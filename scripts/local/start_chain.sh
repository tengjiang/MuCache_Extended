#!/bin/bash
# Start the chain benchmark locally (no Kubernetes, no Dapr, no ZMQ).
#
# Topology:
#   client → service1:3001 → service2:3002 → service3:3003 → service4:3004 → backend:3005
#                ↓                ↓                ↓                ↓               ↓
#             CM1:9001         CM2:9002         CM3:9003         CM4:9004        CM5:9005
#                              (all share Redis at localhost:6379)
#
# Usage:
#   ./scripts/local/start_chain.sh          # start with MuCache CM enabled
#   ./scripts/local/start_chain.sh nocm     # start without CM (baseline)

set -e

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BIN="$REPO_ROOT/bin"
LOGS="$REPO_ROOT/logs/chain"
CM_ADDR_FILE="$REPO_ROOT/experiments/local_cm/chain.txt"
SVC_URL_FILE="$REPO_ROOT/experiments/local_services/chain.txt"

WITH_CM=true
if [[ "${1:-}" == "nocm" ]]; then
    WITH_CM=false
fi

mkdir -p "$LOGS"

# ── helpers ────────────────────────────────────────────────────────────────────

log() { echo "[start_chain] $*"; }

stop_chain() {
    log "Stopping all chain processes..."
    pkill -f "chain_service" 2>/dev/null || true
    pkill -f "chain_backend" 2>/dev/null || true
    pkill -f "bin/cm "       2>/dev/null || true
    sleep 0.5
}

wait_http() {
    local url="$1" name="$2"
    for i in $(seq 1 30); do
        if curl -sf "$url" >/dev/null 2>&1; then
            log "$name is up"
            return 0
        fi
        sleep 0.3
    done
    log "ERROR: $name at $url did not start in time"
    exit 1
}

# ── clean up any previous run ──────────────────────────────────────────────────
stop_chain

# ── Redis check ────────────────────────────────────────────────────────────────
if ! redis-cli ping >/dev/null 2>&1; then
    log "Starting Redis..."
    redis-server --daemonize yes --logfile "$LOGS/redis.log"
    sleep 1
fi
redis-cli flushall >/dev/null  # clean state between runs
log "Redis OK"

# ── Start Cache Managers (one per service) ──────────────────────────────────────
if $WITH_CM; then
    log "Starting Cache Managers..."
    for idx in 1 2 3 4 5; do
        port=$((9000 + idx))
        env NODE_IDX="$idx" REDIS_URL="localhost:6379" \
            "$BIN/cm" -port "$port" -cm_adds "$CM_ADDR_FILE" \
            > "$LOGS/cm${idx}.log" 2>&1 &
        log "  CM${idx} (node_idx=$idx) → :${port}"
    done
    sleep 1
fi

# ── Start microservices ────────────────────────────────────────────────────────
log "Starting microservices..."

SUFFIX=$($WITH_CM && echo "cm" || echo "nocm")

start_svc() {
    local name=$1 svcbin=$2 port=$3 cm_port=$4
    env PORT="$port" \
        CM_URL="http://localhost:$cm_port" \
        REDIS_URL="localhost:6379" \
        SERVICE_URLS_FILE="$SVC_URL_FILE" \
        APP_NAME_NO_UNDERSCORES="$name" \
        "$svcbin" > "$LOGS/${name}.log" 2>&1 &
    log "  $name → :${port}  (CM at :${cm_port}, binary: $(basename $svcbin))"
}

start_svc service1 "$BIN/chain_service1_${SUFFIX}" 3001 9001
start_svc service2 "$BIN/chain_service2_${SUFFIX}" 3002 9002
start_svc service3 "$BIN/chain_service3_${SUFFIX}" 3003 9003
start_svc service4 "$BIN/chain_service4_${SUFFIX}" 3004 9004
start_svc backend  "$BIN/chain_backend_${SUFFIX}"  3005 9005

# ── Wait for all services to be ready ─────────────────────────────────────────
log "Waiting for services..."
wait_http http://localhost:3001/heartbeat "service1"
wait_http http://localhost:3002/heartbeat "service2"
wait_http http://localhost:3003/heartbeat "service3"
wait_http http://localhost:3004/heartbeat "service4"
wait_http http://localhost:3005/heartbeat "backend"

log ""
log "All services up! Chain benchmark ready."
log ""
log "Example requests:"
log "  Read:    curl -s -X POST http://localhost:3001/ro_read      -H 'Content-Type: application/json' -d '{\"k\":1}'"
log "  Write:   curl -s -X POST http://localhost:3001/write        -H 'Content-Type: application/json' -d '{\"k\":1,\"v\":42}'"
log "  HitorMiss: curl -s -X POST http://localhost:3001/ro_hitormiss -H 'Content-Type: application/json' -d '{\"k\":1,\"hit_rate\":0.5}'"
log ""
log "Load test (100 req/s for 5s):"
log "  oha -n 500 -c 10 -m POST -H 'Content-Type: application/json' -d '{\"k\":1}' http://localhost:3001/ro_read"
log ""
log "Logs in: $LOGS/"
log "Stop:    pkill -f chain_service; pkill -f chain_backend; pkill -f 'bin/cm '"
