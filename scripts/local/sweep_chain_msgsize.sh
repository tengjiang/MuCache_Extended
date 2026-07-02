#!/bin/bash
# Local single-node throughput-latency sweep for the chain benchmark,
# one curve per RpcMsgSize (flame frame size). Also runs an HTTP (nocm)
# baseline. All services + daemons + Redis + oha run on this host.
#
# Output: results/chain_msgsize/summary.csv with a msg_size column.
set -e

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$REPO_ROOT"
GO=/usr/local/go/bin/go
RPC_GO="pkg/flame/rpc.go"

SIZES=(${SIZES:-256 512 1024 2048 4096 8192 16384})
CONCURRENCIES=(${CONCURRENCIES:-4 8 16 32 64 96 128})
N_REQUESTS="${N_REQUESTS:-30000}"
RUNS="${RUNS:-2}"
ENDPOINT="http://localhost:3001/ro_read"

OUTDIR="$REPO_ROOT/results/chain_msgsize"
mkdir -p "$OUTDIR"
SUMMARY="$OUTDIR/summary.csv"
echo "series,msg_size,concurrency,requests,p50_ms,p95_ms,p99_ms,rps,success_rate" > "$SUMMARY"

log() { echo "[msgsweep $(date +%H:%M:%S)] $*"; }

# Restore the original RpcMsgSize on exit no matter what.
ORIG_SIZE=$(grep -oP 'const RpcMsgSize = \K[0-9]+' "$RPC_GO")
cleanup() {
    pkill -f chain_service 2>/dev/null || true
    pkill -f chain_backend 2>/dev/null || true
    pkill -f flame_daemon 2>/dev/null || true
    sed -i "s/^const RpcMsgSize = .*/const RpcMsgSize = $ORIG_SIZE/" "$RPC_GO"
    log "Restored RpcMsgSize = $ORIG_SIZE"
}
trap cleanup EXIT

build_flame() {
    for svc in service1 service2 service3 service4 backend; do
        $GO build -tags flame -o "bin/chain_${svc}_flame" "./cmd/chain/${svc}"
    done
}
build_nocm() {
    for svc in service1 service2 service3 service4 backend; do
        $GO build -o "bin/chain_${svc}_nocm" "./cmd/chain/${svc}"
    done
}

populate() {
    for k in $(seq 1 100); do
        curl -s -X POST http://localhost:3001/write \
            -H 'Content-Type: application/json' -d "{\"k\":$k,\"v\":$k}" >/dev/null
    done
}

# run_point <series> <msg_size> <concurrency>
run_point() {
    local series="$1" size="$2" c="$3"
    local files=()
    for r in $(seq 1 "$RUNS"); do
        local f="$OUTDIR/${series}_s${size}_c${c}_run${r}.txt"
        oha -n "$N_REQUESTS" -c "$c" -m POST --no-tui \
            -H 'Content-Type: application/json' -d '{"k":1}' "$ENDPOINT" > "$f" 2>&1 || true
        files+=("$f")
    done
    local row
    row=$(python3 - "${files[@]}" <<'PY'
import sys, re
files = sys.argv[1:]
ansi = re.compile(r'\x1b\[[0-9;]*[mGKHF]|\x1b\[?[0-9]*[lh]')
vals = {'p50':[], 'p95':[], 'p99':[], 'rps':[], 'succ':[]}
for f in files:
    text = ansi.sub('', open(f).read())
    for s in (l.strip() for l in text.splitlines()):
        for key, pat in (('p50', r'50\.00%\s+in\s+(\S+)'),
                         ('p95', r'95\.00%\s+in\s+(\S+)'),
                         ('p99', r'99\.00%\s+in\s+(\S+)'),
                         ('rps', r'Requests/sec:\s+(\S+)')):
            m = re.match(pat, s)
            if m: vals[key].append(float(m.group(1)))
        m = re.match(r'Success rate:\s+(\S+)', s)
        if m: vals['succ'].append(m.group(1))
avg = lambda k: sum(vals[k])/len(vals[k]) if vals[k] else 0
print(f"{avg('p50')*1000:.4f},{avg('p95')*1000:.4f},{avg('p99')*1000:.4f},{avg('rps'):.2f},{vals['succ'][-1] if vals['succ'] else 'N/A'}")
PY
)
    echo "$series,$size,$c,$N_REQUESTS,$row" >> "$SUMMARY"
    printf "    [%-6s size=%-5s c=%-3d] %s\n" "$series" "$size" "$c" "$row"
}

# ── HTTP baseline (nocm) ───────────────────────────────────────────────────────
log "Building nocm baseline..."
build_nocm
log "=== series: http (nocm) ==="
bash scripts/local/start_chain.sh nocm > "$OUTDIR/start_nocm.log" 2>&1
sleep 2; populate; sleep 1
oha -n 2000 -c 16 -m POST --no-tui -H 'Content-Type: application/json' -d '{"k":1}' "$ENDPOINT" >/dev/null 2>&1 || true
for c in "${CONCURRENCIES[@]}"; do run_point http NA "$c"; done
pkill -f chain_service 2>/dev/null || true; pkill -f chain_backend 2>/dev/null || true
sleep 2

# ── flame, one curve per msg_size ──────────────────────────────────────────────
for size in "${SIZES[@]}"; do
    log "=== series: flame  msg_size=$size ==="
    sed -i "s/^const RpcMsgSize = .*/const RpcMsgSize = $size/" "$RPC_GO"
    build_flame
    FLAME_MSG_SIZE="$size" bash scripts/local/start_chain.sh flame > "$OUTDIR/start_flame_${size}.log" 2>&1
    sleep 2; populate; sleep 1
    oha -n 2000 -c 16 -m POST --no-tui -H 'Content-Type: application/json' -d '{"k":1}' "$ENDPOINT" >/dev/null 2>&1 || true
    for c in "${CONCURRENCIES[@]}"; do run_point flame "$size" "$c"; done
    pkill -f chain_service 2>/dev/null || true
    pkill -f chain_backend 2>/dev/null || true
    pkill -f flame_daemon 2>/dev/null || true
    sleep 2
done

log "DONE -> $SUMMARY"
column -ts, "$SUMMARY"
