# 0004. Bounded flock wait and locked startup

Status: accepted
Date: 2026-11-03

## Context
ADR 0001's sidecar flock blocks with no timeout, so one hung holder (a stopped CLI, a stuck NFS mount) blocks every
server read and win. Seeding a blank DB file with `[]` and the first load at startup run without the flock, so
another process's concurrent `RecordWin` can read the half-seeded file and fail that win.

## Decision
Every flock is taken with LOCK_NB in a retry loop (every 10ms, up to 5s) and returns a wrapped timeout error on
expiry. Reads then serve the cached league and log the error; `RecordWin` returns it. `NewPlayerStore` holds
LOCK_EX while it seeds and loads the file.

## Consequences
Easier: a hung lock costs a 5s wait and a logged error, not a frozen server; startup can't race other processes.
Harder: under heavy contention a win can now fail after 5s instead of waiting; the lock file is created at
startup before the first load, so a store open needs a writable directory.
