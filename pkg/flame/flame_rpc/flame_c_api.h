#ifndef FLAME_RPC_FLAME_C_API_H_
#define FLAME_RPC_FLAME_C_API_H_

/*
 * Plain-C API for flame RPC — CGO / FFI bindings.
 *
 * This API is a thin wrapper around the existing:
 *   flame::benchmark::TrustedCopierServiceBenchmark  (client / server side)
 *   flame::benchmark::TCSPoolManager                  (daemon side)
 *   flame::rpc::CounterQueue*, flame::rpc::Doorbell   (inside)
 *
 * Named-shm (shm_open) replaces memfd_create so independent processes can
 * connect by name without fd passing.
 *
 * Model: ONE channel name = ONE bidirectional RPC pipe between a client
 * and a server, mediated by a daemon.  Internally the daemon creates two
 * shm regions: <name>_cd and <name>_ds.
 *
 * Lifecycle:
 *   1. Daemon: flame_daemon_create() + flame_daemon_run() (blocks)
 *   2. Client: flame_client_connect() then send / recv in pairs
 *   3. Server: flame_server_connect() then recv / send in pairs
 */

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct FlameClient_ FlameClient;
typedef struct FlameServer_ FlameServer;
typedef struct FlameDaemon_ FlameDaemon;

/* ── Daemon ───────────────────────────────────────────────────────────────── */

/*
 * Create both shm regions (<name>_cd and <name>_ds) and initialize the
 * TCSPoolManager. Window_size is the per-side buffer count (default 256).
 * Blocking: nonzero = use futex doorbells, 0 = pure polling.
 *
 * Returns NULL on error (e.g. region already exists — unlink first).
 */
FlameDaemon* flame_daemon_create(const char* name,
                                 size_t      msg_size,
                                 uint32_t    window_size,
                                 int         blocking);

/*
 * Run the copy loop (client→server and server→client).
 * Blocks until flame_daemon_stop() is called from another thread.
 */
void flame_daemon_run(FlameDaemon* d);

/* Signal the run loop to exit (safe from another thread). */
void flame_daemon_stop(FlameDaemon* d);

/* Remove <name>_cd and <name>_ds from /dev/shm/. Call after stop. */
void flame_daemon_unlink(FlameDaemon* d);

/* Free the handle (also munmaps both regions). */
void flame_daemon_destroy(FlameDaemon* d);

/* ── Client ───────────────────────────────────────────────────────────────── */

/*
 * Open <name>_cd (daemon must have created it) and attach as the client.
 * msg_size and window_size must match the daemon's configuration.
 */
FlameClient* flame_client_connect(const char* name,
                                  size_t      msg_size,
                                  uint32_t    window_size,
                                  int         blocking);

/*
 * Send `len` bytes (copied into a shm-backed buffer). If `len` > msg_size
 * the call fails. Blocks (or spin-polls) until a send slot is free.
 * Returns 0 on success, -1 on error.
 */
int flame_client_send(FlameClient* c, const void* buf, size_t len);

/*
 * Receive one response. Copies up to `max_len` bytes into `buf` and writes
 * the actual length to *out_len. Blocks until a response arrives.
 * Returns 0 on success, -1 on error.
 */
int flame_client_recv(FlameClient* c, void* buf, size_t max_len, size_t* out_len);

void flame_client_destroy(FlameClient* c);

/* ── Zero-copy slot API (client) ──────────────────────────────────────────
 *
 * Lets the caller write the outgoing message directly into a shm-backed
 * slot, and read incoming messages in place, eliminating the Go↔shm
 * memcpys that flame_client_send / flame_client_recv perform internally.
 *
 * Send lifecycle (per message):
 *   1. void* slot = flame_client_alloc_slot(c);     // writable, msg_size bytes
 *   2. ...fill up to msg_size bytes...
 *   3. flame_client_commit_send(c, slot, len);      // enqueue (TCS sends full frame)
 *
 * Receive lifecycle (per message):
 *   1. void* msg = flame_client_peek_recv(c, &len); // read in place
 *   2. ...consume msg bytes...
 *   3. flame_client_release(c, msg);                // return to pool
 *
 * The slot from alloc_slot is valid until commit_send. The msg from
 * peek_recv is valid until release. Neither side is thread-safe — wrap
 * with the same mutex that protects the channel.
 */
void* flame_client_alloc_slot (FlameClient* c);
int   flame_client_commit_send(FlameClient* c, void* slot, size_t len);
void* flame_client_peek_recv  (FlameClient* c, size_t* out_len);
int   flame_client_release    (FlameClient* c, void* slot);

/* ── Server ───────────────────────────────────────────────────────────────── */

FlameServer* flame_server_connect(const char* name,
                                  size_t      msg_size,
                                  uint32_t    window_size,
                                  int         blocking);

int flame_server_recv(FlameServer* s, void* buf, size_t max_len, size_t* out_len);
int flame_server_send(FlameServer* s, const void* buf, size_t len);

void flame_server_destroy(FlameServer* s);

/* ── Zero-copy slot API (server) ──────────────────────────────────────────
 * Symmetric mirror of the client side; same lifecycle rules apply.
 */
void* flame_server_alloc_slot (FlameServer* s);
int   flame_server_commit_send(FlameServer* s, void* slot, size_t len);
void* flame_server_peek_recv  (FlameServer* s, size_t* out_len);
int   flame_server_release    (FlameServer* s, void* slot);

#ifdef __cplusplus
}
#endif

#endif
