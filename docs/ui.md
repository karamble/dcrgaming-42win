# Desktop presentation

The player-facing brand is FOUR2WIN. Embedded transparent generated logos are
used on the cover and lobby, with a compact wordmark for small headers. Their
alpha and aspect ratio are preserved by both renderers. Bridge registration,
profiles, protocol names and binaries still use `dcr4inarow`.
`make preview` also exports a curated gallery to `artifacts/four2win/`.

The UI is a read-only consumer of detached session snapshots. It never signs
moves, decides outcomes, changes financial terms or authorizes payments.

`render.SetSize` runs on the UI thread before input/drawing. The same geometry
drives hit targets and both GPU/raster canvases. Text measurement, explicit
weights and field clipping use the bundled Go fonts. No web frontend is needed.

The presentation controller replays newly committed entries into a temporary
match, preserving the winning grid even if the live rules have already advanced
to the next round. Drop and result timers use an injected timestamp. Baselines,
including reconnects, never replay old sounds. Input is blocked during pending
submission or presentation; network delivery continues.

Application errors are scoped by table. Current financial observations come
from this peer's refreshed bridge snapshot, not another player's report or the
session's latched `Paid` boolean. Disconnect, missing chain state and failed
refresh mark evidence stale. No wall-clock estimate is presented as a deadline.

Receipts use an additive, versioned JSON envelope around the existing transcript
and abandonment encoding. `reference_roster` is not a trust anchor; obtain the
roster independently from escrow before using `audit.Match`. Abandoned matches
can have complete result metadata but an unfinished move transcript; their
signed abandonment is separate evidence. Payment observations and displayed
round metadata are not signatures or proofs of payment.

Run `make check`, `make ui-check` on a display, and `make preview`. Review
pending, stale, spent and void states as well as the normal opening and victory.
The UI walkthrough and previews use fixtures, not mainnet payments.
