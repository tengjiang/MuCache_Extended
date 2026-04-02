#ifndef FLAME_RPC_C_API_H_
#define FLAME_RPC_C_API_H_

/*
 * Plain-C API for flame RPC — used by Go CGO bindings.
 *
 * Lifecycle:
 *   1. Daemon process calls flame_daemon_create() then flame_daemon_run()
 *      (blocks; run from a dedicated thread or process).
 *   2. Writer process calls flame_writer_connect(), then flame_writer_send().
 *   3. Reader process calls flame_reader_connect(), then flame_reader_recv_loop()
 *      or flame_reader_try_recv() in a polling loop.
 */

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Opaque handles */
typedef struct FlameWriter_ FlameWriter;
typedef struct FlameReader_ FlameReader;
typedef struct FlameDaemon_ FlameDaemon;

/* Callback invoked by flame_reader_recv for each message.
 * Note: msg points into shm; treat as read-only even though not const-qualified
 * (const void* is intentionally avoided to satisfy CGO export constraints). */
typedef void (*FlameRecvCb)(void* msg, size_t msg_size, void* user_data);

/* ── Daemon ───────────────────────────────────────────────────────────────── */

/* Create both shm regions (cd + ds).  Returns NULL on error (check errno). */
FlameDaemon* flame_daemon_create(const char* channel_name,
                                 size_t      msg_size,
                                 size_t      capacity,
                                 int         doorbell);   /* 0 = poll, 1 = doorbell */

/* Run the copy loop — blocks until flame_daemon_stop() is called. */
void flame_daemon_run(FlameDaemon* d);

/* Signal the daemon's run() loop to exit (safe to call from another thread). */
void flame_daemon_stop(FlameDaemon* d);

/* Remove shm names from /dev/shm.  Call after stop(). */
void flame_daemon_unlink(FlameDaemon* d);

/* Free the handle (also calls close() on both regions). */
void flame_daemon_destroy(FlameDaemon* d);

/* ── Writer ───────────────────────────────────────────────────────────────── */

/* Open the cd shm region (daemon must have created it first). */
FlameWriter* flame_writer_connect(const char* channel_name,
                                  size_t      msg_size,
                                  size_t      capacity,
                                  int         doorbell);

/* Copy msg_size bytes from buf into the ring.  Blocks (spin) if full. */
void flame_writer_send(FlameWriter* w, const void* buf);

/* Close the shm mapping and free the handle. */
void flame_writer_destroy(FlameWriter* w);

/* ── Reader ───────────────────────────────────────────────────────────────── */

/* Open the ds shm region (daemon must have created it first). */
FlameReader* flame_reader_connect(const char* channel_name,
                                  size_t      msg_size,
                                  size_t      capacity,
                                  int         doorbell);

/*
 * Block until the next message is ready, invoke cb(msg, msg_size, user_data),
 * then return.  Repeat in a loop to process a stream.
 */
void flame_reader_recv(FlameReader* r, FlameRecvCb cb, void* user_data);

/*
 * Non-blocking: if a message is ready invoke cb and return 1, else return 0.
 */
int flame_reader_try_recv(FlameReader* r, FlameRecvCb cb, void* user_data);

/* Close the shm mapping and free the handle. */
void flame_reader_destroy(FlameReader* r);

#ifdef __cplusplus
} /* extern "C" */
#endif

#endif /* FLAME_RPC_C_API_H_ */
