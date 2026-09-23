# FOUR2WIN

Heads-up four-in-a-row for real stakes on Decred, over Bison Relay.

FOUR2WIN is the player-facing brand. The repository and Go module are
`github.com/karamble/dcrgaming-42win`. Executable names, existing profile paths
and bridge protocol identifiers are unchanged by the repository rename.

Two seats, best of three boards, first to two wins takes the pot. Every move is a
signed entry in one hash-chained log, so a finished match verifies offline from
its transcript and the roster alone — and a player who tells two different
stories about one move publishes their own signing key doing it.

Built on [dcrgaming-sdk](https://github.com/karamble/dcrgaming-sdk). The game
holds no wallet keys and cannot pay anybody: it asks a
[dcrpulse](https://github.com/karamble/dcrpulse) gaming bridge, and a person at
that dashboard approves every payment.

## Building

Requires Go 1.26 and the sibling SDK checkout:

```text
karamble/
  dcrgaming-sdk/
  dcrgaming-42win/
```

```sh
make check      # race tests, vet, both desktop builds
make preview    # every screen to PNG, no display needed
make ui-check   # actual desktop/GPU fixture walkthrough (requires a display)
make play       # the local fixture board
```

## Running it

The illustrated cover prepares local artwork, then Enter, Space or a click
opens the lobby. It never waits for a bridge or imposes an artificial delay.
`--skip-cover` bypasses it; `--settings` and `--dev-board` also skip the cover.
The window starts at 1280×800 and adapts down to 960×640.

Use the mouse to choose a column, or Left/Right and Enter/Space to drop. Tab
and Shift+Tab move between controls; Escape returns to the lobby. Round results
remain visible briefly before the next board; Enter/Space can dismiss a round
presentation. Settings includes sound and reduced-motion toggles, saved in the
profile's `ui.json`. Bridge settings must be disconnected before editing them.

The result screen separates the match outcome from bridge observations.
“Stake spend in progress” and “Stake reported spent” do not promise confirmed
winnings. Payment approvals and recovery remain in dcrpulse → Gaming.

Open **Match receipt** after a result to export the signed transcript, reference
roster, round results, accepted terms, and any signed abandonment. Exports are
unique owner-only JSON files in `APPDATA/exports/`. Their descriptive metadata
is not signed payment evidence. Independently verify moves with `audit.Match`
and a trusted roster obtained from escrow, not solely the exported roster.

It follows Decred's conventions, so it should behave like the node and wallet
beside it. The application directory comes from `dcrutil`, a
`dcr4inarow.conf` is written there on first run, and the command line overrides
it. Each directory is a separate identity, which is how two seats run on one
machine.

```sh
dcr4inarow -A ~/.dcr4inarow-one --settings   # paste the bridge credentials
dcr4inarow -A ~/.dcr4inarow-two --connect    # a second seat, its own identity
```

| Option | |
|---|---|
| `-A`, `--appdata`, `--datadir` | profile directory |
| `-C`, `--configfile` | configuration file |
| `--logdir`, `--maxlogfiles`, `--logsize` | log location and rotation |
| `-d`, `--debuglevel` | `info`, or `SESS=debug,SDK=warn`; `show` lists subsystems |
| `--bridge-config` | credential file (default `APPDATA/bridge.json`) |
| `--connect`, `--settings` | connect at startup, or open bridge settings |
| `--skip-cover` | bypass the illustrated startup screen |

Logging is Decred `slog` with four subsystems — `FOUR` the client, `SESS` the
rules and move log, `SDK` the gaming runtime, `BRDG` the bridge connection —
written to stdout and a rotating `logs/dcr4inarow.log`.

Bridge credentials are pasted in the settings screen and stored unencrypted in
`bridge.json` with owner-only permissions. Protect that file as a bridge
credential; the private key is never shown, logged or passed on a command line.

## What works

- `internal/board`, `internal/match` — the grid and the best-of-three rules.
- `internal/movelog` — signed hash-chained moves, transcripts, and a durable
  signing book that survives a restart.
- `internal/audit` — verify a finished match from its transcript and roster.
- `internal/session` — the SDK `runtime.Rules` implementation. Two peers seat,
  fund, play and settle against an in-process bridge over real mTLS.
- `internal/bridgeconn` — bridge credentials, the network choice, and the mTLS
  connection they prove.
- `internal/appconfig`, `internal/logging` — Decred-style configuration and
  subsystem logging with bounded rotation.
- `internal/tablelobby` — the seating screen's state: what each seat has paid,
  how far its confirmations have come, and what the table is waiting for. Pure,
  so every state can be rendered without a bridge.
- `pkg/render`, `cmd/dcr4inarow` — the lobby, bridge settings, seating screen
  and board, on a GPU canvas and an offline rasteriser that draw the same thing.

## Watching a table form

A table takes minutes to build and the money moves during those minutes, so the
client shows it happening: a six-step rail (invitation, admission bond, roster,
seat draw, stake, ready), a card per seat with each deposit and its confirmation
count, what the table costs, and one line saying what is being waited for.

Two rules it keeps. A deposit counts as verified only when **this** peer checked
the output itself, against a policy depth that exists, with the confirmations
actually in — a peer saying its money is down is not evidence that it is. And
when the bridge goes away the screen stops claiming anything is verified, because
checks made before a disconnect are not checks made now.

`make preview` renders every stage, including the ones that are awkward to reach
on purpose: a bond one confirmation short, an aborted table, stale evidence.
It also renders loading, round/result, financial-status, help and receipt
screens, with representative layouts at 960×640, 1280×800 and 1920×1080.

`make ui-check` opens a temporary fixture window, exercises the real input
handlers and GPU renderer, plays three rounds, and captures application-only
screenshots below `/tmp/dcr4inarow-ui-walkthrough-*`. No bridge or wallet is used.

## What does not

Nothing has run against a real wallet, a real bridge or a real chain. The
`-dev-board` fixture has no log, no stake and no bridge behind it.

**Payout requires both seats to sign.** If settlement fails, each seat can
reclaim its own stake after its lock, less fees. Winnings are not guaranteed.

**An opponent who stops playing cannot lose their stake.** After 12 blocks
without a move you can end the match; it settles void and every seat takes back
what it put in. You recover yours, not theirs — so a seat that is losing can
walk away rather than finish.

[Approved decisions](docs/decisions.md) records what was agreed and why. Where
it and the code disagree, the code is right.
