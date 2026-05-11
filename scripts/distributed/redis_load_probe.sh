#!/bin/bash
# Sample Redis load on N2 to verify it is NOT the bottleneck.
#
# Usage:
#   bash redis_load_probe.sh start <out.csv>   # begin background sampling
#   bash redis_load_probe.sh stop              # stop the sampler
#
# Each sample line: ts,ops_per_sec,connected_clients,used_cpu_sys,used_cpu_user
# ops_per_sec comes from `INFO stats instantaneous_ops_per_sec` — Redis's own
# 1-second EWMA, which is exactly what we care about.
set -e
source "$(dirname "$0")/env.sh"

PIDFILE="/tmp/redis_load_probe.pid"

case "${1:-}" in
start)
    out="${2:?need output csv path}"
    log "Starting redis load probe → $out"
    echo "ts,ops_per_sec,connected_clients,used_cpu_sys,used_cpu_user" > "$out"
    (
        while true; do
            line=$(ssh_n2 "redis-cli -h 127.0.0.1 -p $REDIS_PORT INFO stats clients cpu 2>/dev/null" \
                | tr -d '\r' \
                | awk -F: '
                    /^instantaneous_ops_per_sec:/ {ops=$2}
                    /^connected_clients:/         {cli=$2}
                    /^used_cpu_sys:/              {sys=$2}
                    /^used_cpu_user:/             {usr=$2}
                    END { printf("%s,%s,%s,%s\n", ops, cli, sys, usr) }')
            printf '%s,%s\n' "$(date +%s)" "$line" >> "$out"
            sleep 1
        done
    ) &
    echo $! > "$PIDFILE"
    log "probe pid $(cat $PIDFILE)"
    ;;
stop)
    if [[ -f "$PIDFILE" ]]; then
        kill "$(cat $PIDFILE)" 2>/dev/null || true
        rm -f "$PIDFILE"
        log "redis load probe stopped"
    fi
    ;;
*)
    echo "Usage: $0 {start <csv>|stop}" >&2
    exit 1
    ;;
esac
