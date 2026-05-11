#!/bin/bash
# Throughput–latency sweep for chain (distributed N0/N1/N2).
# Each (mode, concurrency) point is run RUNS times and averaged.
# Endpoint: /ro_read.
set -e

# Set sweep-specific defaults BEFORE sourcing env.sh, which would otherwise
# export N_REQUESTS=3000 (its quick-run default) and shadow our value.
: "${N_REQUESTS:=150000}"
: "${RUNS:=3}"

source "$(dirname "$0")/env.sh"

MODES=(${MODES:-nocm flame})
CONCURRENCIES=(${CONCURRENCIES:-25 50 75 100 125 150 175 200})

OUTDIR="${OUTDIR:-$REPO_ROOT/results/chain_distributed}"
mkdir -p "$OUTDIR"

FRONTEND_URL="http://$N1_PUBLIC_IP:3001"

SUMMARY="$OUTDIR/summary.csv"
echo "mode,concurrency,requests,p50_secs,p95_secs,p99_secs,rps,success_rate,redis_ops_per_sec,redis_cpu_util" > "$SUMMARY"

log "chain sweep: modes=${MODES[*]}  c=${CONCURRENCIES[*]}  n=$N_REQUESTS  runs=$RUNS  → $OUTDIR"

# ── helper: run oha RUNS times, average metrics, append row to summary ────────
# Usage: run_and_avg <mode> <concurrency> <oha-args...>
run_and_avg() {
    local mode="$1" c="$2"; shift 2
    local files=()
    # Bracket the whole concurrency point with Redis snapshots so we can
    # compute average ops/sec and CPU utilization over the run.
    local pre_snap=$(redis_snapshot)
    local t0=$(date +%s.%N)
    for r in $(seq 1 "$RUNS"); do
        local out_file="$OUTDIR/${mode}_c${c}_run${r}.txt"
        log "  run $r/$RUNS  n=$N_REQUESTS c=$c..."
        oha -n "$N_REQUESTS" -c "$c" "$@" > "$out_file" 2>&1 || true
        files+=("$out_file")
    done
    local t1=$(date +%s.%N)
    local post_snap=$(redis_snapshot)
    local redis_stats
    redis_stats=$(python3 - "$pre_snap" "$post_snap" "$t0" "$t1" <<'PYEOF'
import sys
pre = sys.argv[1].split(',')
post = sys.argv[2].split(',')
dt = float(sys.argv[4]) - float(sys.argv[3])
try:
    d_cmd = float(post[0]) - float(pre[0])
    d_user = float(post[2]) - float(pre[2])
    d_sys = float(post[3]) - float(pre[3])
    ops_per_sec = d_cmd / dt if dt > 0 else 0
    cpu_util = (d_user + d_sys) / dt if dt > 0 else 0  # fraction of 1 core
    print(f"{ops_per_sec:.0f},{cpu_util:.3f}")
except Exception:
    print("0,0")
PYEOF
)
    local result
    result=$(python3 - "${files[@]}" <<'PYEOF'
import sys, re
files = sys.argv[1:]
vals = {'p50': [], 'p95': [], 'p99': [], 'rps': [], 'succ': []}
ansi = re.compile(r'\x1b\[[0-9;]*[mGKHF]|\x1b\[?[0-9]*[lh]')
for f in files:
    text = ansi.sub('', open(f).read())
    for line in text.splitlines():
        s = line.strip()
        m = re.match(r'50\.00%\s+in\s+(\S+)', s)
        if m: vals['p50'].append(float(m.group(1)))
        m = re.match(r'95\.00%\s+in\s+(\S+)', s)
        if m: vals['p95'].append(float(m.group(1)))
        m = re.match(r'99\.00%\s+in\s+(\S+)', s)
        if m: vals['p99'].append(float(m.group(1)))
        m = re.match(r'Requests/sec:\s+(\S+)', s)
        if m: vals['rps'].append(float(m.group(1)))
        m = re.match(r'Success rate:\s+(\S+)', s)
        if m: vals['succ'].append(m.group(1))
def avg(k): return sum(vals[k])/len(vals[k]) if vals[k] else 0
print(f"{avg('p50'):.4f},{avg('p95'):.4f},{avg('p99'):.4f},{avg('rps'):.4f},{vals['succ'][-1] if vals['succ'] else 'N/A'}")
PYEOF
)
    local p50 p95 p99 rps succ
    IFS=',' read -r p50 p95 p99 rps succ <<< "$result"
    local r_ops r_cpu
    IFS=',' read -r r_ops r_cpu <<< "$redis_stats"
    echo "$mode,$c,$N_REQUESTS,$p50,$p95,$p99,$rps,$succ,$r_ops,$r_cpu" >> "$SUMMARY"
    printf "    [%s c=%-3d] p50=%s  p95=%s  p99=%s  rps=%s  succ=%s  redis=%s ops/s (%.0f%% core)\n" \
        "$mode" "$c" "${p50:-?}" "${p95:-?}" "${p99:-?}" "${rps:-?}" "${succ:-?}" \
        "${r_ops:-?}" "$(awk "BEGIN{print ($r_cpu)*100}")"
}

for MODE in "${MODES[@]}"; do
    echo ""
    echo "=== chain / $MODE ==="
    bash "$(dirname "$0")/start_redis_N2.sh"

    log "Starting chain services on N1 ($MODE)..."
    ssh_n1 "REDIS_ADDR='$REDIS_ADDR' N1_PUBLIC_IP='$N1_PUBLIC_IP' \
            bash $REPO_ROOT/scripts/distributed/start_chain_N1.sh $MODE" \
        > "$OUTDIR/start_${MODE}.log" 2>&1
    sleep 3

    log "Populating 100 keys..."
    for k in $(seq 1 100); do
        curl -s -X POST "$FRONTEND_URL/write" \
            -H 'Content-Type: application/json' \
            -d "{\"k\":$k,\"v\":$k}" > /dev/null
    done
    sleep 2

    log "Warming up..."
    oha -n 1000 -c 20 -m POST --no-tui \
        -H 'Content-Type: application/json' \
        -d '{"k":1}' "$FRONTEND_URL/ro_read" > /dev/null 2>&1 || true
    sleep 2

    for c in "${CONCURRENCIES[@]}"; do
        log "Concurrency $c ($RUNS runs)..."
        run_and_avg "$MODE" "$c" -m POST --no-tui \
            -H 'Content-Type: application/json' \
            -d '{"k":1}' "$FRONTEND_URL/ro_read"
    done

    ssh_n1 "bash $REPO_ROOT/scripts/distributed/stop_N1.sh" > /dev/null 2>&1 || true
    sleep 3
done

echo ""
log "DONE."
column -ts, "$SUMMARY"
