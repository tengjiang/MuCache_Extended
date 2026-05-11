#!/bin/bash
# Throughput–latency sweep for boutique (distributed N0/N1/N2).
# Each (mode, concurrency) point is run RUNS times and averaged.
#
# Run on Node 0:
#   bash scripts/distributed/sweep_boutique.sh
#   N_REQUESTS=200000 CONCURRENCIES="25 50 100 200" bash scripts/distributed/sweep_boutique.sh
set -e

# Set sweep-specific defaults BEFORE sourcing env.sh, which would otherwise
# export N_REQUESTS=3000 (its quick-run default) and shadow our value.
: "${N_REQUESTS:=150000}"
: "${RUNS:=3}"

source "$(dirname "$0")/env.sh"

MODES=(${MODES:-nocm flame})
CONCURRENCIES=(${CONCURRENCIES:-25 50 75 100 125 150 175 200})

OUTDIR="${OUTDIR:-$REPO_ROOT/results/boutique_distributed}"
mkdir -p "$OUTDIR"

FRONTEND_URL="http://$N1_PUBLIC_IP:4100"
PRODUCT_URL="http://$N1_PUBLIC_IP:4106"
CURRENCY_URL="http://$N1_PUBLIC_IP:4103"

SUMMARY="$OUTDIR/summary.csv"
echo "mode,concurrency,requests,p50_secs,p95_secs,p99_secs,rps,success_rate" > "$SUMMARY"

log "Sweep config:"
log "  modes         = ${MODES[*]}"
log "  concurrencies = ${CONCURRENCIES[*]}"
log "  N_REQUESTS    = $N_REQUESTS"
log "  RUNS          = $RUNS"
log "  N1            = $NODE1_HOST ($N1_PUBLIC_IP)"
log "  N2            = $NODE2_HOST ($N2_INTERNAL_IP)"
log "  output dir    = $OUTDIR"

# ── helper: run oha RUNS times, average metrics, append row to summary ────────
# Usage: run_and_avg <mode> <concurrency> <oha-args...>
run_and_avg() {
    local mode="$1" c="$2"; shift 2
    local files=()
    for r in $(seq 1 "$RUNS"); do
        local out_file="$OUTDIR/${mode}_c${c}_run${r}.txt"
        log "  run $r/$RUNS  n=$N_REQUESTS c=$c..."
        oha -n "$N_REQUESTS" -c "$c" "$@" > "$out_file" 2>&1 || true
        files+=("$out_file")
    done
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
    echo "$mode,$c,$N_REQUESTS,$p50,$p95,$p99,$rps,$succ" >> "$SUMMARY"
    printf "    [%s c=%-3d] p50=%s  p95=%s  p99=%s  rps=%s  succ=%s\n" \
        "$mode" "$c" "${p50:-?}" "${p95:-?}" "${p99:-?}" "${rps:-?}" "${succ:-?}"
}

for MODE in "${MODES[@]}"; do
    echo ""
    echo "================================================================"
    echo "  boutique / $MODE"
    echo "================================================================"

    bash "$(dirname "$0")/start_redis_N2.sh"

    log "Starting boutique services on N1 ($MODE)..."
    ssh_n1 "REDIS_ADDR='$REDIS_ADDR' N1_PUBLIC_IP='$N1_PUBLIC_IP' \
            bash $REPO_ROOT/scripts/distributed/start_boutique_N1.sh $MODE" \
        > "$OUTDIR/start_${MODE}.log" 2>&1
    sleep 3

    log "Populating boutique data..."
    bash "$REPO_ROOT/scripts/local/populate_boutique.sh" \
        "$FRONTEND_URL" "$PRODUCT_URL" "$CURRENCY_URL" 100 > /dev/null
    sleep 3

    log "Warming up..."
    oha -n 1000 -c 20 -m POST --no-tui \
        -H 'Content-Type: application/json' \
        -d '{"user_id":"user_0","user_currency":"USD"}' \
        "$FRONTEND_URL/ro_home" > /dev/null 2>&1 || true
    sleep 2

    for c in "${CONCURRENCIES[@]}"; do
        log "Concurrency $c ($RUNS runs)..."
        run_and_avg "$MODE" "$c" -m POST --no-tui \
            -H 'Content-Type: application/json' \
            -d '{"user_id":"user_0","user_currency":"USD"}' \
            "$FRONTEND_URL/ro_home"
    done

    log "Stopping services on N1..."
    ssh_n1 "bash $REPO_ROOT/scripts/distributed/stop_N1.sh" > /dev/null 2>&1 || true
    sleep 3
done

echo ""
log "DONE. Summary:"
column -ts, "$SUMMARY"
log "Per-run oha output: $OUTDIR/*.txt"
log "CSV:                $SUMMARY"
