# dcr4inarow — approved decisions

Recorded 2026-09-18 from the project owner's instructions. This document records
what was decided and why. It does not define behavior: the implementation is
authoritative, and where the two disagree the code is right and this file is stale.

## Product

- Name: **dcr4inarow**. Go module: `github.com/karamble/dcr4inarow`.
- Go/Ebiten native desktop game, same stack as dcrstakewars.
- **Two seats maximum.** Standard 7×6 board.
- Every match is staked. No solo mode, no hotseat, no unpaid public demo.
  Offline fixtures and simnet builds are engineering tools, not game modes.
- Built on `dcrgaming-sdk`. dcrpulse holds the wallet and the Bison Relay
  identity and is the only financial authority; the game asks and never pays.

## Match format

- **Best of three boards. The first seat to two board wins takes the pot.**
- A drawn board scores nothing. If no seat reaches two wins after three boards,
  the match settles `runtime.Outcome{Void: true}` and each seat takes back its
  own stake. A game nobody won says so rather than inventing an even split.
- Openers: board 1 seat 0, board 2 seat 1, board 3 the seat that won its board
  in fewer moves. Equal move counts, or no wins, fall back to seat 0.
- **Corrected 2026-09-18, during implementation.** The rule was first agreed as
  "board 1 the table creator", derived by the game from the invitation. Nothing
  on the wire says who the creator is: `schema.Invite` carries terms and no
  host, dcrpulse creates tables rather than games, and both peers are asked to
  accept the same invitation. Neither side can compute it, so the rule was not
  implementable. Seat 0 is the only answer both peers reach alike, and it is a
  better one: the seat draw is a block hash nobody could see when they chose
  their key, and it is settled before anybody funds. It costs no extra wait,
  because seating has already happened by the time a board is played.
- **No block wait inside a match.** Nothing in the match schedule depends on a
  future block. Board 3's opening is earned by play, not drawn.

Rejected, and why: a beacon draw or a pie rule for board 3 would each have made
the decider wait on a block or add a swap exchange. Four-in-a-row is a solved
game, so a fixed opener is a real edge; earning board 3 removes it rather than
allocating it.

## Seating and timing

- Seating uses the SDK beacon: `membership.BeaconHeight = Until + 1`. Seat order
  is drawn from a block hash nobody could see when they chose their key.
- The beacon costs **one block**. The admission deadline `Until` costs the rest:
  `seatIfReady` waits for the height even when the table is already full.
  Heads-up tables therefore set a short admission window. `Until` is the lever.
- Invitation terms — buy-in, seats, admission deadline, refund lock — come from
  dcrpulse's New table dialog. The game must not substitute its own.
- `FundingDeadline` and `BondingDeadline` are deadlines, not waits. A table both
  seats answer promptly never feels them.

## Auditability

- Every move is a signed entry in one hash-chained log per match. A completed
  match verifies offline from the transcript and the roster alone, by anyone.
- The roster verified against is the one the escrow committed to, never group
  chat membership.
- Signatures use `forfeit` positional nonces: exactly one signature may ever
  exist at one `Position{Match, Domain, Seq}`. Two different messages at one
  position hand over the signer's private key.
- **One monotonic move counter across all three boards.** The board index is a
  field in the entry and never a reset of the counter. A counter that starts
  over re-signs a used position with a different column and publishes the key.
- `DomainEntry` for moves, `DomainCheckpoint` for per-board results.
  `forfeit.Domain` is a closed allowlist, so game-specific domains would be an
  additive SDK change. Not required for v1.
- The `LogKey`'s `Book` must be durable and must survive a restart. An
  in-memory book catches the mistake within one process and no further.
- The log key is per match and separate from the session key holding the stake.
  It is expected to become public the moment its owner misbehaves.

## Money

- Payout is cooperative N-of-N. A refusing signer can prevent a payout. Each
  seat can reclaim its own unspent deposit after its lock, less fees. This must
  be disclosed before funding, along with which seat opens which boards.
- **v1 enforcement is evidence-only.** Equivocation stays provable and
  publishable, but the SDK's `Seize`, `Accuse` and `Release` verbs no longer
  exist and nothing spends a bond on a proof. The levers are refusing to
  co-sign and publishing the evidence.
- A bridge-verified bond sweep — dcrpulse checking the two-signature proof
  itself and offering the operator a spend — remains possible later. Freeze the
  entry encoding now so that change needs no wire break.
- `membership.Terms` still carries `AccuseFeeAtoms` and `ForfeitBondAtoms` with
  nothing consuming them. Set them deliberately rather than by accident.
- No backward compatibility and no legacy migration. There is no deployed state.

## When a seat stops playing

Decided 2026-09-18, after the implementation showed what was actually available.

- A move is owed by a **block height**: the last entry's height plus
  `MoveDeadlineBlocks`, 12 blocks or roughly an hour. Heights rather than
  clocks, for the reason the SDK's own admission deadline is a height — a
  deadline two machines read differently decides money by whose clock ran fast.
  It is frozen by game version and derived from the log, so there is nothing to
  negotiate and nothing to disagree about.
- A match has its own ceiling, `MatchDeadlineBlocks`, 144 blocks or roughly
  twelve hours from its first move. A per-move deadline does not bound a match:
  a best-of-three is up to 126 moves, and two seats could keep a table open for
  days without either ever being late. Past the ceiling either seat may end the
  match whosever turn it is, and the abandonment says so — an expiry accuses
  nobody, and is a different signed statement from a stall.
- The locks are chosen against that ceiling, not picked: refund and admission
  bond both sit at 288 blocks, roughly a day, twice the longest a match can run.
  A stake whose refund matured mid-match could be taken back by its owner,
  leaving a pot that can no longer pay.
- Past that height the waiting seat **may** publish a signed abandonment. May,
  not must: a seat that is ahead can prefer to wait for its opponent to come
  back and finish rather than void a match it is winning.
- An abandonment is signed under `forfeit.DomainLeaving` — the SDK's own word
  for a seat getting up — at the sequence number of the move that was owed. Two
  different abandonments at one position publish the signer's key, exactly as
  equivocating about a move does. It is about the log, not in it: the move never
  arrived, so there is no entry to chain to.
- An abandoned match settles `Outcome{Void: true}`. It is adopted rather than
  argued with: a peer that is behind and one lying about being behind look
  identical, and the outcome is the same either way.
- **Nothing is forfeited, because nothing can be.** `Seize`, `Accuse` and the
  claim ladder are gone from the SDK, so no verb moves another seat's bond. A
  rule phrased as a penalty would be theatre.
- Void is the one outcome a returning staller has reason to sign: it costs them
  nothing and returns their stake. If they never return, each seat reclaims its
  own stake after the refund lock, needing nobody.

**The accepted consequence.** A losing seat is better off stalling than
finishing: walking away converts a loss into a void and returns its stake. That
is the direct consequence of evidence-only enforcement, not a flaw in the rule,
and it must be disclosed before funding alongside the payout warning:
**an opponent who stops playing cannot lose their stake; you recover yours, not
theirs.** Closing it needs bridge-verified seizure, which is deferred.

**Not yet established:**

- Buy-in, admission bond and honesty bond amounts.

## Implementation order

1. Board and move log, offline. No network, no money. Tests that matter:
   equivocation at one position recovers the key; a restart cannot re-sign a
   used position; a third party holding only the roster verifies a whole match.
2. `runtime.Rules` against `pkg/gaming/bridgetest` — two peers, real mTLS, a
   whole match, asserting exact frame counts and zero resync traffic. Then its
   failure injection: unreachable, hold, refuse.
3. The money path: admission bond, seat draw, stake, play, settle or void.
   Each obligation asks the bridge exactly once; an unreachable bridge never
   becomes a refusal.
4. Ebiten client: lobby, table, board, transcript viewer, bridge settings,
   `-datadir` profiles, two-peer local demo.
5. Two independent wallets, bridges and games on simnet before any mainnet drill.

Seat tags are frozen at first commit and never touched:
`4inarow/session/v1`, `4inarow/log/v1`, `4inarow/bond/v1`.
