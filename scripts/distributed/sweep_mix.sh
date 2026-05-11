#!/bin/bash
# Mixed-workload, rate-based throughput–latency sweep using vegeta.
#
# One script handles all three benchmarks. The mix per benchmark is defined
# inline below (build_targets) — each line in the resulting targets file is
# a single request specification consumed by vegeta in round-robin order,
# so the line proportions ARE the request mix.
#
# Usage:
#   BENCH=chain bash scripts/distributed/sweep_mix.sh
#   BENCH=boutique bash scripts/distributed/sweep_mix.sh
#   BENCH=hotel    bash scripts/distributed/sweep_mix.sh
#   BENCH=chain RATES="500 1000 2000" DURATION=2s bash scripts/distributed/sweep_mix.sh
#
# Output:
#   results/<bench>_mix_<YYYYMMDD-HHMMSS>/
#       summary.csv             (mode,rate,actual_rps,p50,p95,p99,success,...)
#       targets_<mode>.txt      (the mixed targets used)
#       <mode>_r<rate>.json     (per-point vegeta JSON output)
#       <mode>_r<rate>.bin      (raw vegeta result binary)
#
# We hit the SAME endpoints regardless of flame/nocm — the mode only changes
# how intra-N1 RPC works, not the HTTP-facing frontends.
set -e

: "${RUNS:=1}"
: "${DURATION:=4s}"
source "$(dirname "$0")/env.sh"

# Need vegeta on $PATH. We install it to $HOME/bin in this branch.
export PATH="$HOME/bin:$PATH"
command -v vegeta >/dev/null || { echo "vegeta not on PATH"; exit 1; }

BENCH="${BENCH:?BENCH must be chain|boutique|hotel}"
MODES=(${MODES:-nocm flame})

# ── benchmark-specific config ────────────────────────────────────────────────
case "$BENCH" in
chain)
    DEFAULT_RATES="500 1000 2000 4000 8000 12000 16000 20000 25000 30000 40000 50000 60000 80000"
    FRONTEND_URL="http://$N1_PUBLIC_IP:3001"
    START_SCRIPT="start_chain_N1.sh"
    POPULATE_FN() {
        for k in $(seq 0 99); do
            curl -s -X POST "$FRONTEND_URL/write" \
                -H 'Content-Type: application/json' \
                -d "{\"k\":$k,\"v\":$k}" > /dev/null
        done
    }
    # 80% reads / 20% writes, 5-hop chain.
    build_targets() {
        local out="$1"
        : > "$out.read.body"; printf '{"k":1}' > "$out.read.body"
        : > "$out.write.body"; printf '{"k":1,"v":1}' > "$out.write.body"
        : > "$out"
        for _ in 1 2 3 4 5 6 7 8; do  # 8/10 = 80% reads
            {
                echo "POST $FRONTEND_URL/ro_read"
                echo "Content-Type: application/json"
                echo "@$out.read.body"
                echo ""
            } >> "$out"
        done
        for _ in 1 2; do              # 2/10 = 20% writes
            {
                echo "POST $FRONTEND_URL/write"
                echo "Content-Type: application/json"
                echo "@$out.write.body"
                echo ""
            } >> "$out"
        done
    }
    ;;

boutique)
    DEFAULT_RATES="500 1000 2000 4000 6000 8000 10000 12000 14000 16000 18000 22000 26000 32000"
    FRONTEND_URL="http://$N1_PUBLIC_IP:4100"
    CART_URL="http://$N1_PUBLIC_IP:4101"
    PRODUCT_URL="http://$N1_PUBLIC_IP:4106"
    CURRENCY_URL="http://$N1_PUBLIC_IP:4103"
    START_SCRIPT="start_boutique_N1.sh"
    POPULATE_FN() {
        bash "$REPO_ROOT/scripts/local/populate_boutique.sh" \
            "$FRONTEND_URL" "$PRODUCT_URL" "$CURRENCY_URL" 100 > /dev/null
    }
    # 40 home / 25 browse / 15 view_cart / 10 checkout / 10 add_item.
    build_targets() {
        local out="$1"
        # product IDs are populated as "p0".."p99" by populate_boutique.sh,
        # NOT "product_0". Match that or the catalog returns errors.
        printf '{"user_id":"user_0","catalog_size":10}'              > "$out.home.body"
        printf '{"product_id":"p0"}'                                 > "$out.browse.body"
        printf '{"user_id":"user_0"}'                                > "$out.viewcart.body"
        printf '{"user_id":"user_0","product_id":"p0","quantity":1}' > "$out.additem.body"
        cat > "$out.checkout.body" <<EOF_BODY
{"user_id":"user_0","user_currency":"USD","address":{"street_address":"1 a","city":"b","state":"c","country":"d","zip_code":94100},"email":"a@b.com","credit_card":{"card_number":"4111111111111111","card_type":"visa","expiration_month":1,"expiration_year":2030}}
EOF_BODY
        : > "$out"
        emit() {
            local n="$1" url="$2" body="$3"
            for _ in $(seq 1 "$n"); do
                {
                    echo "POST $url"
                    echo "Content-Type: application/json"
                    echo "@$body"
                    echo ""
                } >> "$out"
            done
        }
        emit 8 "$FRONTEND_URL/ro_home"          "$out.home.body"      # 40%
        emit 5 "$FRONTEND_URL/ro_browse_product" "$out.browse.body"   # 25%
        emit 3 "$FRONTEND_URL/ro_view_cart"     "$out.viewcart.body"  # 15%
        emit 2 "$FRONTEND_URL/checkout"         "$out.checkout.body"  # 10%
        emit 2 "$CART_URL/add_item"             "$out.additem.body"   # 10%
    }
    ;;

hotel)
    DEFAULT_RATES="200 500 1000 1500 2000 3000 4000 5000 6000 7000 8000 9000 10000 12000"
    FRONTEND_URL="http://$N1_PUBLIC_IP:4000"
    USER_URL="http://$N1_PUBLIC_IP:4005"
    START_SCRIPT="start_hotel_N1.sh"
    POPULATE_FN() {
        bash "$REPO_ROOT/scripts/local/populate_hotel.sh" \
            "$FRONTEND_URL" "$USER_URL" 100 20 > /dev/null
    }
    # 80 search / 20 reservation. Search is the heavy multi-service read.
    build_targets() {
        local out="$1"
        # populate_hotel.sh names hotels "0".."99" and users "username0".."username19"
        # with passwords "password0".."password19". Match those exactly.
        printf '{"in_date":"2024-01-01","out_date":"2024-01-02","location":"city0"}' \
            > "$out.search.body"
        printf '{"hotel_id":"1","in_date":"2024-01-01","out_date":"2024-01-02","rooms":1,"username":"username0","password":"password0"}' \
            > "$out.reserve.body"
        : > "$out"
        for _ in 1 2 3 4 5 6 7 8; do
            {
                echo "POST $FRONTEND_URL/ro_search_hotels"
                echo "Content-Type: application/json"
                echo "@$out.search.body"
                echo ""
            } >> "$out"
        done
        for _ in 1 2; do
            {
                echo "POST $FRONTEND_URL/reservation"
                echo "Content-Type: application/json"
                echo "@$out.reserve.body"
                echo ""
            } >> "$out"
        done
    }
    ;;

*) echo "Unknown BENCH=$BENCH"; exit 1 ;;
esac

RATES=(${RATES:-$DEFAULT_RATES})
STAMP="$(date +%Y%m%d-%H%M%S)"
OUTDIR="${OUTDIR:-$REPO_ROOT/results/${BENCH}_mix_${STAMP}}"
mkdir -p "$OUTDIR"

SUMMARY="$OUTDIR/summary.csv"
echo "mode,target_rate,actual_rps,p50_secs,p95_secs,p99_secs,success_rate,redis_ops_per_sec,redis_cpu_util" > "$SUMMARY"

log "${BENCH} MIX sweep: modes=${MODES[*]}  rates=${RATES[*]}  dur=$DURATION  runs=$RUNS  → $OUTDIR"

# ── per-point runner: rate-based open-loop with redis bracketing ──────────────
run_point() {
    local mode="$1" rate="$2" targets="$3"
    local pre=$(redis_snapshot)
    local t0=$(date +%s.%N)
    local out_bin="$OUTDIR/${mode}_r${rate}.bin"
    : > "$out_bin"
    for r in $(seq 1 "$RUNS"); do
        local tmp="$OUTDIR/${mode}_r${rate}_run${r}.bin"
        # 5s per-request timeout keeps dead points cheap; under healthy load
        # we never get close to 5s anyway.
        vegeta attack \
            -targets="$targets" \
            -rate="${rate}/1s" \
            -duration="$DURATION" \
            -timeout=5s \
            -max-workers=8000 \
            -output="$tmp" 2>/dev/null
        cat "$tmp" >> "$out_bin"
        rm -f "$tmp"
    done
    local t1=$(date +%s.%N)
    local post=$(redis_snapshot)
    local rpt_json="$OUTDIR/${mode}_r${rate}.json"
    vegeta report -type=json < "$out_bin" > "$rpt_json"

    python3 - "$rpt_json" "$pre" "$post" "$t0" "$t1" >> "$SUMMARY" <<PYEOF
import json, sys
with open(sys.argv[1]) as f: r = json.load(f)
pre  = sys.argv[2].split(',')
post = sys.argv[3].split(',')
dt   = float(sys.argv[5]) - float(sys.argv[4])
def to_s(ns): return ns / 1e9
lats = r.get('latencies', {})
p50 = to_s(lats.get('50th', 0))
p95 = to_s(lats.get('95th', 0))
p99 = to_s(lats.get('99th', 0))
rps = r.get('throughput', 0)
succ = r.get('success', 0) * 100
try:
    d_cmd  = float(post[0]) - float(pre[0])
    d_user = float(post[2]) - float(pre[2])
    d_sys  = float(post[3]) - float(pre[3])
    rops = d_cmd / dt if dt > 0 else 0
    rcpu = (d_user + d_sys) / dt if dt > 0 else 0
except Exception:
    rops, rcpu = 0, 0
# pull rate target from filename (stable enough for our needs)
import os
fn = os.path.basename(sys.argv[1])
rate = int(fn.split('_r')[1].split('.')[0])
mode = fn.split('_')[0]
print(f"{mode},{rate},{rps:.2f},{p50:.5f},{p95:.5f},{p99:.5f},{succ:.2f}%,{rops:.0f},{rcpu:.3f}")
PYEOF
    # print last summary line for live progress
    tail -1 "$SUMMARY"
}

# ── outer loop: per mode, bring up services, sweep rates, tear down ──────────
for MODE in "${MODES[@]}"; do
    echo ""
    echo "================================================================"
    echo "  $BENCH MIX / $MODE"
    echo "================================================================"

    bash "$(dirname "$0")/start_redis_N2.sh"

    log "Starting $BENCH services on N1 ($MODE)..."
    ssh_n1 "REDIS_ADDR='$REDIS_ADDR' N1_PUBLIC_IP='$N1_PUBLIC_IP' \
            bash $REPO_ROOT/scripts/distributed/$START_SCRIPT $MODE" \
        > "$OUTDIR/start_${MODE}.log" 2>&1
    sleep 3

    log "Populating $BENCH data..."
    POPULATE_FN
    sleep 2

    targets_file="$OUTDIR/targets_${MODE}.txt"
    build_targets "$targets_file"
    log "Targets file ($(grep -c '^POST' "$targets_file") request lines): $targets_file"

    log "Warming up..."
    vegeta attack -targets="$targets_file" -rate=200/1s -duration=2s -timeout=10s \
        > /dev/null 2>&1 || true
    sleep 1

    for rate in "${RATES[@]}"; do
        log "  rate=$rate/s ..."
        run_point "$MODE" "$rate" "$targets_file"
    done

    ssh_n1 "bash $REPO_ROOT/scripts/distributed/stop_N1.sh" > /dev/null 2>&1 || true
    sleep 2
done

log "DONE. Summary at $SUMMARY"
column -ts, "$SUMMARY"
