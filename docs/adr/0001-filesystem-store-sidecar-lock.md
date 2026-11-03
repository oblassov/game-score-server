# 0001. Filesystem store sidecar lock

Status: accepted
Date: 2026-11-03

## Context
The CLI and the web server share one JSON league file (bug 37): reads are served from a per-process cache and
the only lock is an in-process RWMutex, so readers miss other processes' wins and concurrent writers lose updates.
Writes are temp-file + rename (tape.go), so `flock` on the DB file locks an inode that the next write replaces.
mtime checks miss writes inside one timestamp tick; the file is tiny, so re-reading it is cheap. go.mod has no
golang.org/x/sys. Bug 38 (symlinked DB, read-only dir) asked this ADR to fix the write policy too.

## Decision
Each store holds `<db>.lock` open next to the DB and takes `syscall.Flock` on it, LOCK_SH for GetLeague and
GetPlayerScore (which re-read the file, no cache) and LOCK_EX around RecordWin's read-modify-rename, inside the
existing RWMutex (flock does not exclude goroutines sharing one fd). A lock file that cannot be created fails
the constructor (no read-only fallback). Symlinks are not resolved: a symlinked DB is unsupported (bug 38, Won't
fix). Non-unix builds keep the in-process lock only.

## Consequences
Easier: the CLI and server see each other's wins and never lose one; no new dependency.
Harder: every read hits the disk and a lock syscall; a stray `.lock` file sits next to the DB; a read-only
directory now fails at startup instead of on the first win; Windows gets no cross-process safety.
