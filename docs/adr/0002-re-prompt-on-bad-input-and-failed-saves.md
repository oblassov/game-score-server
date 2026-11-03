# 0002. Re-prompt on bad input and failed saves

Status: accepted
Date: 2026-11-03

## Context
A typo in the player count or winner line ended the CLI run or closed the /ws socket (1003), and a win that
couldn't be saved was only logged because `Game.Finish` returned nothing. Names were checked only at the
transport layer, and POST accepted control characters, `/` and any length, answering a bad name with an empty 400.

## Decision
The CLI and /ws log a bad count, a bad winner or a failed save, send the reason to the user, and ask again (no
retry cap; the /ws read deadlines bound a connection). Only EOF or a closed socket ends a game.
A saved win ends the /ws game with close frame 1000 (normal closure). `Game.Finish`
returns an error. Names are trimmed and rejected (400 with a one-line reason) when blank, not valid UTF-8, or
over 64 bytes, or when they contain control characters or `/`. The stores trim names too, so no caller can skip it.

## Consequences
Easier: one mistake doesn't lose a game or a win, and clients see why a name was refused. Harder: the CLI and /ws
flows become loops; clients that expected the 1003 close after a bad count must read the reason message instead.
