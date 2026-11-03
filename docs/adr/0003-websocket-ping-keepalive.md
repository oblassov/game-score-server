# 0003. WebSocket ping keepalive

Status: accepted
Date: 2026-11-03

## Context
While /ws waits for the winner, its only bound is a 24h read deadline (the longest game is 4.5h at 22 players).
A client that disappears without closing the socket holds a goroutine and a connection for up to 24h. Browsers
answer WebSocket pings automatically, so a keepalive needs no client change.

## Decision
The server pings every 30s and allows 60s for a pong; each pong pushes the read deadline to
min(now + 60s, the 24h winner deadline). A missed pong ends the read, and the game ends like a closed socket.

## Consequences
Easier: dead clients are released within about a minute; the 24h cap stays as the hard bound for live ones.
Harder: a ping writer runs alongside the blind alerts and must share the write lock and write deadline;
the timings are package values that tests shorten.
