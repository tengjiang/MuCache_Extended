#!/bin/bash
# Sweep chain over multiple read/write ratios at one rate ladder.
# Output: results/chain_rw_<ratio>_<stamp>/ per (ratio).
#
# Each ratio sweep runs both modes (nocm + flame) at a moderate per-mode
# rate ladder that brackets the saturation knee. Writes dominate state.SetState,
# which always hits Redis; reads can be served from MuCache's cache. So as
# READ_PCT drops, we expect:
#   - Redis ops per sec rises
#   - flame's per-request advantage shrinks (writes are not the bottleneck
#     case where 4 intra-N1 RPC hops dominate)
#   - both modes' peak throughput drops
set -e
source "$(dirname "$0")/env.sh"

export PATH="$HOME/bin:$PATH"

RATIOS=(${RATIOS:-100 80 50 20 0})
: "${DURATION:=15s}"
# Wider rate range — different ratios saturate at different rates.
: "${RATES:=500 2000 5000 10000 15000 20000 25000 30000}"
export DURATION RATES

STAMP="$(date +%Y%m%d-%H%M%S)"
OUT_ROOT="$REPO_ROOT/results/chain_rw_${STAMP}"
mkdir -p "$OUT_ROOT"

log "chain R/W ratio sweep: ratios=${RATIOS[*]}  rates=$RATES  dur=$DURATION"
log "output root: $OUT_ROOT"

for ratio in "${RATIOS[@]}"; do
    log "==================================================================="
    log " READ_PCT=$ratio  (reads=$((ratio/10))  writes=$((10-ratio/10)))"
    log "==================================================================="
    OUTDIR="$OUT_ROOT/r${ratio}" \
    READ_PCT="$ratio" \
    BENCH=chain \
        bash "$(dirname "$0")/sweep_mix.sh"
done

log "DONE. Sweep root: $OUT_ROOT"
