# MuCache_Extended — Agent Notes (distributed branch)

## What this repo is

MuCache_Extended benchmarks a shared-memory RPC transport called **flame** (futex doorbell +
`/dev/shm` ring buffers) against plain loopback HTTP (**nocm**) for Go microservice chains.
Three benchmarks: **chain** (5-hop read), **boutique** (6-service fan-out), **hotel** (Redis-heavy search).

## 3-node distributed setup

| Node | Hostname | Experiment IP | Role |
|------|----------|--------------|------|
| **N0** | `c220g5-111231.wisc.cloudlab.us` | `10.10.1.1` | Orchestrator + `oha` load generator |
| **N1** | `c220g5-111311.wisc.cloudlab.us` | `10.10.1.2` | All microservices + flame daemons |
| **N2** | `c220g5-111306.wisc.cloudlab.us` | `10.10.1.3` | Redis backend only |

Traffic flow:
- N0 → N1: HTTP over public network (oha sends requests to N1's frontend port)
- N1 → N2: Redis TCP over experiment LAN (10.10.1.x)
- N1 intra (flame mode): `/dev/shm` shared-memory RPC via `flame_daemon` (futex doorbells, `Blocking: true`)
- N1 intra (nocm mode): loopback HTTP between services

All scripts run **from N0**. SSH to N1/N2 is key-based (no password).

## Repos required on both N0 and N1

```
/mydata/MuCache_Extended/   (branch: distributed)
/mydata/flame-benchmark/    (branch: tcs_api)
```

## Build

On **N1** (services + flame_daemon):
```bash
cd /mydata/flame-benchmark && make -j          # → bin/flame_daemon
cd /mydata/MuCache_Extended && make -j         # → bin/* (~40 binaries)
```

On **N0**: only `oha` and `redis-server` needed (no Go build required).

### Runtime library dependency (important)
All MuCache binaries (even nocm) link `libzmq.so.5` via `pkg/cm → github.com/pebbe/zmq4`.
N1 needs: `libzmq5`, `libpgm-5.3.so.0`, `libnorm.so.1`.

If apt is broken on N1, copy from N0:
```bash
for lib in libzmq.so.5 libpgm-5.3.so.0 libnorm.so.1; do
  scp /lib/x86_64-linux-gnu/${lib}* tengj@c220g5-111311.wisc.cloudlab.us:/tmp/
done
ssh tengj@c220g5-111311.wisc.cloudlab.us "sudo cp /tmp/libzmq* /tmp/libpgm* /tmp/libnorm* /usr/local/lib/ && sudo ldconfig"
```

## Preflight check

```bash
cd /mydata/MuCache_Extended
bash scripts/distributed/check_setup.sh
```

## Running sweeps

All sweep scripts run from N0 in `/mydata/MuCache_Extended`:

```bash
# Individual benchmarks
bash scripts/distributed/sweep_chain.sh
bash scripts/distributed/sweep_boutique.sh
bash scripts/distributed/sweep_hotel.sh

# All three sequentially
bash scripts/distributed/sweep_chain.sh && \
bash scripts/distributed/sweep_boutique.sh && \
bash scripts/distributed/sweep_hotel.sh
```

### Sweep parameters
- `N_REQUESTS=150000` (default; set BEFORE sourcing env.sh via `: "${N_REQUESTS:=150000}"`)
- `RUNS=3` (each concurrency point averaged over 3 oha runs)
- `CONCURRENCIES="25 50 75 100 125 150 175 200"` (default)
- `MODES="nocm flame"` (default)

**Critical**: env.sh exports `N_REQUESTS=3000` for quick runs. Sweep scripts set the default
BEFORE sourcing env.sh using `: "${N_REQUESTS:=150000}"` so the default takes effect.
If you add a new sweep script, follow the same pattern.

### API endpoints benchmarked
| Benchmark | Endpoint | Payload |
|-----------|----------|---------|
| chain | `POST /ro_read` | `{"k":1}` |
| boutique | `POST /ro_home` | `{"user_id":"user_0","user_currency":"USD"}` |
| hotel | `POST /ro_search_hotels` | `{"in_date":"2024-01-01","out_date":"2024-01-02","location":"city0"}` |

## Plotting

```bash
python3 scripts/plot_results.py                    # all 3 benchmarks
python3 scripts/plot_results.py --outdir results/figs
```

Outputs to `results/figs/`: `tput_latency.png`, `throughput.png`, `latency_p50.png`,
`latency_p99.png`, `combined.png`.

Requires: `python3-matplotlib`, `python3-pandas` (install via apt).

## Results summary (150K req × 3 runs, May 2026)

| Benchmark | http (nocm) peak rps | flame peak rps | speedup |
|-----------|---------------------|----------------|---------|
| chain | ~28.4K | ~57.1K | **2.0×** |
| boutique | ~14.7K | ~19.2K | **1.36×** |
| hotel | ~3.5K | ~3.5K | **1.0×** |

Hotel is Redis-bound: each search request fires **2N+2 Redis GETs** (N = hotels in location,
~100 in benchmark). Rate and Reservation services loop with individual `GetState()` calls instead
of `MGET`. Flame doesn't help because the bottleneck is the N1→N2 TCP hop, not inter-service RPC.

## Key files changed on this branch

| File | What changed |
|------|-------------|
| `pkg/latency/latency.go` | New: atomic latency tracking, prints LATENCY REPORT every 5s |
| `pkg/state/state.go` | Added `latency.Record("redis_get")`, `redis_mget`, `json_unmarshal` |
| `pkg/invoke/invoke.go` | Added `latency.Record("http_rpc_call")` |
| `pkg/invoke/flame.go` | Added `latency.Record("flame_rpc_call")` + build tags |
| `pkg/wrappers/wrappers.go` | Added `latency.Record("wrapper_handler")` |
| `pkg/flame/channel.go` | Added `//go:build flame` build tags |
| `pkg/flame/rpc.go` | Added build tags; `Blocking: true` (futex doorbells) |
| `scripts/distributed/` | Full 3-node sweep infrastructure (new) |
| `scripts/plot_results.py` | Throughput-latency plotting script (new) |
| `.gitignore` | Added `!scripts/distributed/env.sh` negation |

## Known gotchas

1. **env.sh N_REQUESTS conflict**: env.sh exports `N_REQUESTS=3000`. New sweep scripts must
   set their default with `: "${N_REQUESTS:=150000}"` BEFORE `source env.sh`.

2. **oha ANSI codes**: oha 1.4.5 emits ANSI escape codes even to files. All `parse_and_append`
   functions strip them with `sed 's/\x1b\[[0-9;]*[mGKHF]//g'` before awk parsing.

3. **libzmq on N1**: nocm binaries still need libzmq at runtime (transitive dep via pkg/cm).
   Always verify with `ldd bin/chain_service1_nocm | grep "not found"` after syncing binaries.

4. **flame intra-N1 only**: flame_daemon runs on N1 and handles shm RPC between N1 services.
   N0→N1 and N1→N2 are always plain TCP regardless of mode.

5. **Hotel is always Redis-bound**: don't expect flame to help hotel. The bottleneck is
   sequential Redis GETs in Rate/Reservation service loops (2N ops per search, N=hotels).
