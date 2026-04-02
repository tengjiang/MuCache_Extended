#ifndef FLAME_RPC_MUCACHE_MESSAGES_H_
#define FLAME_RPC_MUCACHE_MESSAGES_H_

/*
 * Fixed-layout C structs for MuCache wrapper→CM messages (Start, End, Inv).
 * Shared between C++ (flame RPC) and Go (CGO).
 *
 * Design constraints
 * ------------------
 *  - All variable-length fields are null-terminated and bounded.
 *  - Structs are naturally aligned (no __attribute__((packed))).
 *  - A single channel carries all three types; msg_size must be ≥
 *    sizeof(FlameEndMsg), the largest struct (≤ 1024 bytes with defaults).
 *  - Adjust FLAME_* limits to trade memory vs. generality.
 *    The sizes below are tuned for the chain benchmark where keys are short,
 *    calls return small JSON values, and dep counts are low.
 *
 * Recommended msg_size: FLAME_MUCACHE_MSG_SIZE (1024 bytes).
 */

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* ── Tunable limits ──────────────────────────────────────────────────────── */

#define FLAME_APP_NAME_MAX   64   /* service name, e.g. "service1"           */
#define FLAME_CALLARGS_MAX   20   /* FNV-32 hex hash: "a1b2c3d4\0" = 9 chars */
#define FLAME_KEY_MAX        64   /* Redis key, e.g. "key1\0"                */
#define FLAME_RETVAL_MAX    512   /* JSON-encoded return value               */
#define FLAME_KEY_DEPS_MAX    8   /* max key deps per EndRequest             */
#define FLAME_CALL_DEPS_MAX   8   /* max call deps per EndRequest            */

/* ── Discriminant ─────────────────────────────────────────────────────────── */

typedef uint8_t FlameMsgType;
#define FLAME_MSG_START   ((FlameMsgType)1)
#define FLAME_MSG_END     ((FlameMsgType)2)
#define FLAME_MSG_INV_KEY ((FlameMsgType)3)

/* ── 1. StartRequest ──────────────────────────────────────────────────────── */

typedef struct {
    FlameMsgType type;                      /* = FLAME_MSG_START */
    uint8_t      _pad[7];
    char         callargs[FLAME_CALLARGS_MAX];
    char         app_name[FLAME_APP_NAME_MAX];
    /* total: 8 + 20 + 64 = 92 bytes */
} FlameStartMsg;

/* ── 2. EndRequest ────────────────────────────────────────────────────────── */

typedef struct {
    FlameMsgType type;                      /* = FLAME_MSG_END */
    uint8_t      n_key_deps;                /* number of valid key_deps entries */
    uint8_t      n_call_deps;               /* number of valid call_deps entries */
    uint8_t      _pad[5];
    char         callargs[FLAME_CALLARGS_MAX];
    char         caller[FLAME_APP_NAME_MAX]; /* service that is ending */
    char         key_deps[FLAME_KEY_DEPS_MAX][FLAME_KEY_MAX];
    char         call_deps[FLAME_CALL_DEPS_MAX][FLAME_CALLARGS_MAX];
    uint32_t     retval_len;                /* actual byte length of retval */
    char         retval[FLAME_RETVAL_MAX];
    /* total: 8 + 20 + 64 + 8*64 + 8*20 + 4 + 512 = 8+20+64+512+160+4+512 = 1280 bytes
     * → FLAME_MUCACHE_MSG_SIZE should be ≥ 1280; use 1280 or 1536 for alignment */
} FlameEndMsg;

/* ── 3. InvalidateKeyRequest ──────────────────────────────────────────────── */

typedef struct {
    FlameMsgType type;                      /* = FLAME_MSG_INV_KEY */
    uint8_t      from_cm;                   /* 0 = from wrapper, 1 = from another CM */
    uint8_t      _pad[6];
    char         key[FLAME_KEY_MAX];
    /* total: 8 + 64 = 72 bytes */
} FlameInvKeyMsg;

/* ── Union for dispatch ───────────────────────────────────────────────────── */

typedef union {
    FlameMsgType  type;
    FlameStartMsg start;
    FlameEndMsg   end;
    FlameInvKeyMsg inv_key;
} FlameMucacheMsg;

/*
 * Recommended channel msg_size (covers FlameEndMsg, rounded up to 64-byte
 * cache-line multiple).
 */
#define FLAME_MUCACHE_MSG_SIZE 1280

#ifdef __cplusplus
} /* extern "C" */
#endif

#endif /* FLAME_RPC_MUCACHE_MESSAGES_H_ */
