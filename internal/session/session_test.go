package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/karamble/dcr4inarow/internal/audit"
	"github.com/karamble/dcr4inarow/internal/manifest"
	"github.com/karamble/dcr4inarow/internal/movelog"
	"github.com/karamble/dcr4inarow/internal/session"
	"github.com/karamble/dcrgaming-sdk/pkg/forfeit"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/schema"
	"github.com/karamble/dcrgaming-sdk/pkg/gaming/wire"
	"github.com/karamble/dcrgaming-sdk/pkg/membership"
	sdk "github.com/karamble/dcrgaming-sdk/pkg/runtime"
)

const (
	sid     = "abcdef01"
	matchID = "641be69ec6100361ff3e35357355c9d16c8a4a2f0f5b4a2b9cbb6a1d5f0e7c33"
)

// openerWinsIn7 is a vertical win for whoever opens the board.
var openerWinsIn7 = []uint8{0, 1, 0, 1, 0, 1, 0}

// stakeAtoms is what each seat's stub reports as funded.
const stakeAtoms int64 = 5_000_000

// wiring is the two peers and the frames between them. It stands in for the
// bridge: the runtime that frames and relays a message, without being one.
type wiring struct {
	mu       sync.Mutex
	games    map[uint32]*session.Game
	frames   int
	settled  []sdk.Outcome
	height   int64
	bondsBad bool
	unstaked bool
	// unfunded puts this seat's stake back before it was paid.
	unfunded bool
	// keys are the peers' own log keys, so a test can sign as the other seat.
	keys [2]*forfeit.LogKey
}

// stakeCheck lets a test put the table back before its stakes confirmed.
func (w *wiring) stakeCheck() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.unstaked {
		return "confirming"
	}
	return "verified"
}

func (w *wiring) tip() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.height == 0 {
		return 900
	}
	return w.height
}

func (w *wiring) mineTo(h int64) {
	w.mu.Lock()
	w.height = h
	w.mu.Unlock()
}

func (w *wiring) deliver(t *testing.T, from uint32, kind schema.Kind, body []byte) {
	t.Helper()
	w.mu.Lock()
	w.frames++
	targets := make(map[uint32]*session.Game, len(w.games))
	for seat, g := range w.games {
		if seat != from {
			targets[seat] = g
		}
	}
	w.mu.Unlock()
	for seat, g := range targets {
		if err := g.Handle(context.Background(), sdk.Message{
			Match: sid, GCID: "gc", From: "peer", Kind: kind, Body: body,
		}); err != nil {
			t.Fatalf("seat %d rejected a move from seat %d: %v", seat, from, err)
		}
	}
}

func (w *wiring) sent() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.frames
}

// stub is one peer's view of the runtime: the facade the SDK exposes for
// exactly this, answered from a table that was already seated.
type stub struct {
	t     *testing.T
	seat  uint32
	key   *forfeit.LogKey
	seats map[uint32][]byte
	net   *wiring
}

// Send encodes the body the way the runtime does, which is the whole point of
// standing in for it: a stub that passed the game's own value straight through
// would hide the encoding entirely.
func (s *stub) Send(_ context.Context, match string, kind schema.Kind, body any, _ wire.Class) error {
	if match != sid {
		s.t.Fatalf("a message went to table %q", match)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		s.t.Fatalf("encode body: %v", err)
	}
	s.net.deliver(s.t, s.seat, kind, raw)
	return nil
}
func (s *stub) Settle(_ context.Context, _ string, out sdk.Outcome) error {
	s.net.mu.Lock()
	defer s.net.mu.Unlock()
	s.net.settled = append(s.net.settled, out)
	return nil
}
func (s *stub) Funded(_ string, _ uint32) (string, int64, bool) {
	s.net.mu.Lock()
	defer s.net.mu.Unlock()
	if s.net.unfunded {
		return "", 0, false
	}
	return "txid:0", stakeAtoms, true
}
func (s *stub) Terms(string) membership.Terms { return membership.Terms{Seats: 2} }
func (s *stub) Snapshot(string) (sdk.TableSnapshot, error) {
	snap := sdk.TableSnapshot{Phase: "seated", Seats: map[uint32]string{0: "a", 1: "b"}}
	snap.Record.Payouts = map[uint32]string{0: "pay0", 1: "pay1"}
	for seat := uint32(0); seat < 2; seat++ {
		snap.Deposits = append(snap.Deposits,
			sdk.DepositStatus{Purpose: "seatbond", Seat: seat, Check: "verified", Confirmations: 2, RequiredConfirmations: 2},
			sdk.DepositStatus{Purpose: "stake", Seat: seat, Check: s.net.stakeCheck(), Confirmations: 1, RequiredConfirmations: 1},
		)
	}
	return snap, nil
}
func (s *stub) CheckAdmissionBonds(context.Context, string) error {
	if s.net.bondsBad {
		return errBonds
	}
	return nil
}
func (s *stub) RefreshDeposits(context.Context, string) (sdk.TableSnapshot, error) {
	return s.Snapshot("")
}
func (s *stub) Fund(context.Context, string) error { return nil }
func (s *stub) PayoutFor(string) string            { return "DsPayoutDestination" }

var errBonds = errors.New("an admission bond is not on chain")

func (s *stub) Chain(context.Context) (sdk.Chain, error) {
	return sdk.Chain{Height: s.net.tip()}, nil
}
func (s *stub) BlockHash(context.Context, uint32) (string, error) { return "", nil }
func (s *stub) Seat(string) (uint32, bool)                        { return s.seat, true }
func (s *stub) Seats(string) (map[uint32][]byte, bool)            { return s.seats, true }
func (s *stub) LogSeats(string) (map[uint32][]byte, bool)         { return s.seats, true }
func (s *stub) LogKey(string) (*forfeit.LogKey, error)            { return s.key, nil }
func (s *stub) MatchID(string) (string, bool)                     { return matchID, true }

// seated carries what a test needs to rebuild a peer over its own profile.
type profiles struct {
	dirs  [2]string
	keys  [2]*forfeit.LogKey
	seats map[uint32][]byte
}

// tableWithDirs is table, plus the directories and keys, for restart tests.
func tableWithDirs(t *testing.T) (*wiring, [2]*session.Game, profiles) {
	t.Helper()
	net, games, _ := table(t)
	return net, games, profiles{dirs: lastDirs, keys: net.keys, seats: lastSeats}
}

// lastDirs and lastSeats are what table() most recently built. A package-level
// handoff rather than a wider signature, so the dozen existing call sites of
// table() stay as they are.
var (
	lastDirs  [2]string
	lastSeats map[uint32][]byte
)

// table seats two peers that talk to each other.
func table(t *testing.T) (*wiring, [2]*session.Game, movelog.Roster) {
	t.Helper()
	net := &wiring{games: map[uint32]*session.Game{}, height: 900}
	var keys [2]*forfeit.LogKey
	for i := range keys {
		p, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			t.Fatalf("generate key: %v", err)
		}
		k, err := forfeit.LogKeyFrom(p, matchID)
		if err != nil {
			t.Fatalf("log key: %v", err)
		}
		keys[i] = k
	}
	seats := map[uint32][]byte{
		0: keys[0].Public().SerializeCompressed(),
		1: keys[1].Public().SerializeCompressed(),
	}
	net.keys = keys
	lastSeats = seats
	// Both games exist before either speaks: the rules handshake is an
	// exchange, and a frame sent before the other seat is listening is a
	// frame nobody hears.
	var games [2]*session.Game
	for seat := range games {
		dir := t.TempDir()
		lastDirs[seat] = dir
		g, err := session.New(dir)
		if err != nil {
			t.Fatalf("new game: %v", err)
		}
		t.Cleanup(func() { g.Close() })
		g.Bind(&stub{t: t, seat: uint32(seat), key: keys[seat], seats: seats, net: net})
		games[seat] = g
		net.games[uint32(seat)] = g
	}
	for seat, g := range games {
		g.Seated(context.Background(), sid, seats)
		if err := g.Prepare(context.Background(), sid); err != nil {
			t.Fatalf("seat %d prepare: %v", seat, err)
		}
	}
	for seat, g := range games {
		if err := g.Start(context.Background(), sid); err != nil {
			t.Fatalf("seat %d start: %v", seat, err)
		}
	}
	// One more turn of the chain loop, which is what freezes the pot once the
	// rules are agreed and every stake is confirmed.
	for seat, g := range games {
		if err := g.Prepare(context.Background(), sid); err != nil {
			t.Fatalf("seat %d prepare: %v", seat, err)
		}
	}
	// Setting the table is not gameplay; per-test frame counts start here.
	net.mu.Lock()
	net.frames = 0
	net.mu.Unlock()

	roster := movelog.Roster{0: seats[0], 1: seats[1]}
	return net, games, roster
}

// playBoard plays a column list, asking whoever is to move to make each move.
func playBoard(t *testing.T, games [2]*session.Game, cols []uint8) {
	t.Helper()
	for i, col := range cols {
		view, ok := games[0].View(sid)
		if !ok {
			t.Fatal("seat 0 has no table")
		}
		if view.Done {
			t.Fatalf("move %d: the match ended early", i)
		}
		if err := games[view.Turn].Play(context.Background(), sid, col); err != nil {
			t.Fatalf("move %d into column %d: %v", i, col, err)
		}
	}
}

func TestTwoPeersPlayAWholeMatchAndAgree(t *testing.T) {
	net, games, roster := table(t)
	for i := 0; i < 3; i++ {
		playBoard(t, games, openerWinsIn7)
	}

	first, ok := games[0].View(sid)
	if !ok {
		t.Fatal("seat 0 has no table")
	}
	second, ok := games[1].View(sid)
	if !ok {
		t.Fatal("seat 1 has no table")
	}
	if !first.Done || !second.Done {
		t.Fatalf("the peers disagree that the match ended: %v and %v", first.Done, second.Done)
	}
	if first.Won != second.Won || first.Winner != second.Winner {
		t.Fatalf("the peers disagree on the result: seat %d/%v and seat %d/%v",
			first.Winner, first.Won, second.Winner, second.Won)
	}
	if !first.Won || first.Winner != 0 {
		t.Fatalf("seat %d was paid (won %v); seat 0 opened boards one and three", first.Winner, first.Won)
	}

	// Same history on both sides, byte for byte.
	a, err := games[0].Transcript(sid)
	if err != nil {
		t.Fatalf("transcript: %v", err)
	}
	b, err := games[1].Transcript(sid)
	if err != nil {
		t.Fatalf("transcript: %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("the two peers hold different histories of the same match")
	}

	// And a stranger reaches the same verdict from it.
	audited, err := audit.Match(a, roster, session.OpenerSeat)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	winner, won, done := audited.Outcome()
	if !done || !won || winner != first.Winner {
		t.Fatalf("the audit reached (%d, %v, %v), the players reached seat %d", winner, won, done, first.Winner)
	}

	if net.sent() != len(openerWinsIn7)*3 {
		t.Fatalf("%d frames carried %d moves; a move is one frame and nothing else is sent",
			net.sent(), len(openerWinsIn7)*3)
	}
}

func TestAMoveOutOfTurnIsRefusedBeforeItIsSigned(t *testing.T) {
	net, games, _ := table(t)
	if err := games[1].Play(context.Background(), sid, 0); err == nil {
		t.Fatal("the seat that does not open played first")
	}
	if net.sent() != 0 {
		t.Fatalf("%d frames were sent for a move that was refused", net.sent())
	}
}

// A move into a full column would be refused by the other seat, and a signature
// over it would burn the position the replacement move needs.
func TestAnIllegalMoveIsRefusedBeforeItIsSigned(t *testing.T) {
	net, games, _ := table(t)
	for i := 0; i < 6; i++ {
		view, _ := games[0].View(sid)
		if err := games[view.Turn].Play(context.Background(), sid, 0); err != nil {
			t.Fatalf("filling column 0, move %d: %v", i, err)
		}
	}
	before := net.sent()
	view, _ := games[0].View(sid)
	if err := games[view.Turn].Play(context.Background(), sid, 0); err == nil {
		t.Fatal("a seventh piece went into a six-row column")
	}
	if net.sent() != before {
		t.Fatal("a refused move still put a frame on the wire")
	}
}

// A peer that is handed a move signed by its own seat is being told what it
// did. It has its own record of that, and the two could differ, so it refuses
// rather than adopting somebody else's account of its own move.
func TestAMoveClaimingThisPeersOwnSeatIsRefused(t *testing.T) {
	_, games, roster := table(t)
	if err := games[0].Play(context.Background(), sid, 3); err != nil {
		t.Fatalf("opening move: %v", err)
	}
	transcript, err := games[0].Transcript(sid)
	if err != nil {
		t.Fatalf("transcript: %v", err)
	}
	chain, err := movelog.Unmarshal(transcript, roster)
	if err != nil {
		t.Fatalf("read back the transcript: %v", err)
	}
	entries := chain.Entries()
	if len(entries) != 1 || entries[0].Seat != 0 {
		t.Fatalf("expected one move by seat 0, got %d entries", len(entries))
	}
	body, err := movelog.EncodeEntry(&entries[0])
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	err = games[0].Handle(context.Background(), sdk.Message{
		Match: sid, GCID: "gc", From: "peer", Kind: session.KindMove, Body: body,
	})
	if err == nil {
		t.Fatal("a peer adopted a move attributed to its own seat")
	}
}

func TestAnUnknownMessageKindIsRefused(t *testing.T) {
	_, games, _ := table(t)
	err := games[0].Handle(context.Background(), sdk.Message{Match: sid, Kind: "m.chat", Body: []byte("{}")})
	if err == nil {
		t.Fatal("a message this game does not define was handled")
	}
}

func TestAStateSummaryIsAnsweredWithNothingHappening(t *testing.T) {
	_, games, _ := table(t)
	st := games[0].State(context.Background())
	if len(st.Tables) != 1 || st.Summary == "" {
		t.Fatalf("state was %+v, want one table and a summary", st)
	}
}

func TestTheWinnerIsPaidEverythingTheTableHolds(t *testing.T) {
	net, games, _ := table(t)
	for i := 0; i < 3; i++ {
		playBoard(t, games, openerWinsIn7)
	}
	for seat, g := range games {
		if err := g.Settle(context.Background(), sid); err != nil {
			t.Fatalf("seat %d settling: %v", seat, err)
		}
	}

	net.mu.Lock()
	defer net.mu.Unlock()
	if len(net.settled) != 2 {
		t.Fatalf("%d payouts were proposed; both peers propose their own", len(net.settled))
	}
	for i, out := range net.settled {
		if out.Void {
			t.Fatalf("proposal %d was void after a decided match", i)
		}
		// Every seat is named, including the one paid nothing: an explicit
		// zero is the difference between "paid nothing" and "not considered",
		// and the runtime checks the shares against the seats it holds.
		if len(out.Shares) != 2 {
			t.Fatalf("proposal %d names %d seats, want both", i, len(out.Shares))
		}
		if out.Shares[0] != 2*stakeAtoms || out.Shares[1] != 0 {
			t.Fatalf("proposal %d divided %v, want the whole pot of %d to seat 0",
				i, out.Shares, 2*stakeAtoms)
		}
		var total int64
		for _, a := range out.Shares {
			total += a
		}
		if total != 2*stakeAtoms {
			t.Fatalf("proposal %d totals %d and the table holds %d", i, total, 2*stakeAtoms)
		}
	}
}

func TestSettlingAnUnfinishedMatchIsRefused(t *testing.T) {
	net, games, _ := table(t)
	if err := games[0].Settle(context.Background(), sid); err == nil {
		t.Fatal("a payout was proposed for a match still being played")
	}
	net.mu.Lock()
	defer net.mu.Unlock()
	if len(net.settled) != 0 {
		t.Fatalf("%d payouts were proposed before the match ended", len(net.settled))
	}
}

func TestSettlingTwiceProposesOnce(t *testing.T) {
	net, games, _ := table(t)
	for i := 0; i < 3; i++ {
		playBoard(t, games, openerWinsIn7)
	}
	for i := 0; i < 3; i++ {
		if err := games[0].Settle(context.Background(), sid); err != nil {
			t.Fatalf("settle %d: %v", i, err)
		}
	}
	net.mu.Lock()
	defer net.mu.Unlock()
	if len(net.settled) != 1 {
		t.Fatalf("one seat proposed %d payouts for one match", len(net.settled))
	}
}

// The stall rule. Nothing here punishes anybody, because nothing can: what it
// buys is an agreed moment to stop waiting.

// tick runs the driver's chain loop on both peers. A received abandonment is
// held until this runs and the chain has passed its deadline.
func chainLoop(t *testing.T, games [2]*session.Game) {
	t.Helper()
	for seat, g := range games {
		if err := g.Prepare(context.Background(), sid); err != nil {
			t.Fatalf("seat %d prepare: %v", seat, err)
		}
	}
}

func TestAbandoningBeforeTheDeadlineIsRefused(t *testing.T) {
	net, games, _ := table(t)
	if err := games[0].Play(context.Background(), sid, 3); err != nil {
		t.Fatalf("opening move: %v", err)
	}
	deadline, ok := games[0].Deadline(sid)
	if !ok {
		t.Fatal("a table with a move on it has no deadline")
	}
	net.mineTo(int64(deadline) - 1)
	if err := games[0].Abandon(context.Background(), sid); err == nil {
		t.Fatal("a seat was given up on one block before its deadline")
	}
}

func TestAbandoningOnYourOwnTurnIsRefusedInsideTheMatchDeadline(t *testing.T) {
	net, games, _ := table(t)
	if err := games[0].Play(context.Background(), sid, 3); err != nil {
		t.Fatalf("opening move: %v", err)
	}
	if err := games[1].Play(context.Background(), sid, 4); err != nil {
		t.Fatalf("reply: %v", err)
	}
	// Past the move deadline but well inside the match's own. It is seat 0's
	// move again, so seat 0 is the one holding things up.
	view, _ := games[0].View(sid)
	net.mineTo(int64(view.Deadline) + 1)
	if err := games[0].Abandon(context.Background(), sid); err == nil {
		t.Fatal("a seat abandoned the match while it was the one owing a move")
	}
}

func TestAStallPastTheDeadlineVoidsTheMatch(t *testing.T) {
	net, games, _ := table(t)
	if err := games[0].Play(context.Background(), sid, 3); err != nil {
		t.Fatalf("opening move: %v", err)
	}
	deadline, _ := games[0].Deadline(sid)
	net.mineTo(int64(deadline))

	// Seat 1 never moves. Seat 0 gives up, and seat 1 adopts it only once its
	// own chain loop confirms the deadline has passed.
	if err := games[0].Abandon(context.Background(), sid); err != nil {
		t.Fatalf("abandon: %v", err)
	}
	chainLoop(t, games)
	for seat, g := range games {
		v, ok := g.View(sid)
		if !ok {
			t.Fatalf("seat %d has no table", seat)
		}
		if !v.Abandoned || !v.Done || v.Won {
			t.Fatalf("seat %d sees abandoned=%v done=%v won=%v; an abandoned match is over with no winner",
				seat, v.Abandoned, v.Done, v.Won)
		}
		if v.Stalled != 1 {
			t.Fatalf("seat %d blames seat %d; seat 1 was the one to move", seat, v.Stalled)
		}
	}

	for seat, g := range games {
		if err := g.Settle(context.Background(), sid); err != nil {
			t.Fatalf("seat %d settling: %v", seat, err)
		}
	}
	net.mu.Lock()
	defer net.mu.Unlock()
	if len(net.settled) != 2 {
		t.Fatalf("%d payouts were proposed, want one from each seat", len(net.settled))
	}
	for i, out := range net.settled {
		if !out.Void {
			t.Fatalf("proposal %d divided %v; an abandoned match pays nobody", i, out.Shares)
		}
	}
}

// A seat that came back after its opponent gave up cannot carry on as though
// nothing happened.
func TestAMoveAfterAnAbandonmentIsRefused(t *testing.T) {
	net, games, _ := table(t)
	if err := games[0].Play(context.Background(), sid, 3); err != nil {
		t.Fatalf("opening move: %v", err)
	}
	deadline, _ := games[0].Deadline(sid)
	net.mineTo(int64(deadline))
	if err := games[0].Abandon(context.Background(), sid); err != nil {
		t.Fatalf("abandon: %v", err)
	}
	chainLoop(t, games)
	if err := games[1].Play(context.Background(), sid, 4); err == nil {
		t.Fatal("a seat played on into a match that had been given up on")
	}
}

// A match has its own ceiling, because a move deadline alone does not bound
// one: two seats can both be slow without either ever being late, and a stake
// whose refund matured mid-match can be taken back.
func TestPastTheMatchDeadlineEitherSeatMayEndIt(t *testing.T) {
	net, games, _ := table(t)
	if err := games[0].Play(context.Background(), sid, 3); err != nil {
		t.Fatalf("opening move: %v", err)
	}
	if err := games[1].Play(context.Background(), sid, 4); err != nil {
		t.Fatalf("reply: %v", err)
	}
	// It is seat 0's turn, so before the match deadline seat 0 may not
	// abandon: it is the one holding things up.
	view, _ := games[0].View(sid)
	net.mineTo(int64(view.Deadline) + 1)
	if err := games[0].Abandon(context.Background(), sid); err == nil {
		t.Fatal("a seat abandoned while owing a move, inside the match deadline")
	}
	if view.MatchDeadline <= view.Deadline {
		t.Fatalf("the match deadline (%d) is not beyond a move deadline (%d)",
			view.MatchDeadline, view.Deadline)
	}
	// Past the match deadline it no longer matters whose turn it is.
	net.mineTo(int64(view.MatchDeadline))
	if err := games[0].Abandon(context.Background(), sid); err != nil {
		t.Fatalf("the match was out of time and could not be ended: %v", err)
	}
	chainLoop(t, games)
	after, _ := games[1].View(sid)
	if !after.Abandoned || after.Won {
		t.Fatalf("the other seat sees abandoned=%v won=%v", after.Abandoned, after.Won)
	}
}

// The numbers have to hold together: a match must be unable to outlive the
// refund lock its stakes sit behind.
func TestAMatchCannotOutliveItsRefundLock(t *testing.T) {
	if session.MatchDeadlineBlocks >= session.RefundBlocks {
		t.Fatalf("a match may run %d blocks behind a %d-block refund lock; a seat could take its stake back mid-match",
			session.MatchDeadlineBlocks, session.RefundBlocks)
	}
	if session.MoveDeadlineBlocks >= session.MatchDeadlineBlocks {
		t.Fatalf("a single move may take %d blocks of a %d-block match",
			session.MoveDeadlineBlocks, session.MatchDeadlineBlocks)
	}
}

// A seated roster is agreement on who is at the table, not evidence that
// anybody paid. dcrstakewars checks every seat's admission bond on chain before
// it starts a match, and so does this.
func TestATableIsNotPlayableUntilItsAdmissionBondsAreChecked(t *testing.T) {
	net := &wiring{games: map[uint32]*session.Game{}, height: 900, bondsBad: true}
	p, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	key, err := forfeit.LogKeyFrom(p, matchID)
	if err != nil {
		t.Fatalf("log key: %v", err)
	}
	seats := map[uint32][]byte{
		0: key.Public().SerializeCompressed(),
		1: key.Public().SerializeCompressed(),
	}

	g, err := session.New(t.TempDir())
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	t.Cleanup(func() { g.Close() })
	g.Bind(&stub{t: t, seat: 0, key: key, seats: seats, net: net})
	g.Seated(context.Background(), sid, seats)

	if g.Ready(sid) {
		t.Fatal("a table reported ready before its bonds were checked")
	}
	if err := g.Play(context.Background(), sid, 3); err == nil {
		t.Fatal("a move was signed for a table whose admission bonds were never confirmed")
	}
	if err := g.Prepare(context.Background(), sid); err == nil {
		t.Fatal("preparing a table with a missing admission bond succeeded")
	}

	net.mu.Lock()
	net.bondsBad = false
	net.mu.Unlock()
	if err := g.Prepare(context.Background(), sid); err != nil {
		t.Fatalf("prepare after the bonds confirmed: %v", err)
	}
	if !g.Ready(sid) {
		t.Fatal("a verified table does not report ready")
	}
	// Verified is not playable: the rules handshake and the stakes are their
	// own gates, checked by TestPlayableRequiresTheWholeLadder.
	if ok, _ := g.Playable(sid); ok {
		t.Fatal("a table with no rules handshake reported playable")
	}
}

// Funding is offered only when every precondition holds, and each one has to
// be able to withhold it on its own. Offering it early is how a stake is paid
// into a table that never forms.
func TestCanFundRequiresTheWholeLadder(t *testing.T) {
	net, games, _ := table(t)

	// Funded already: there is nothing to ask for, and asking would be how a
	// stake gets paid twice.
	if ok, why := games[0].CanFund(sid); ok {
		t.Fatalf("funding was offered for a stake already paid: %s", why)
	}

	net.mu.Lock()
	net.unfunded = true
	net.mu.Unlock()
	if ok, why := games[0].CanFund(sid); !ok {
		t.Fatalf("a seated, verified, agreed table with no stake refused funding: %s", why)
	}
}

func TestPlayableRequiresTheWholeLadder(t *testing.T) {
	net, games, _ := table(t)
	if ok, why := games[0].Playable(sid); !ok {
		t.Fatalf("a fully funded, agreed table is not playable: %s", why)
	}
	net.mu.Lock()
	net.unstaked = true
	net.mu.Unlock()
	if err := games[0].Prepare(context.Background(), sid); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	ok, why := games[0].Playable(sid)
	if ok {
		t.Fatal("an unstaked table is playable")
	}
	if why == "" {
		t.Fatal("an unplayable table gives no reason")
	}
	if err := games[0].Play(context.Background(), sid, 3); err == nil {
		t.Fatal("a move was signed for a table whose stakes are not confirmed")
	}
}

// Both seats must be playing the same game, and saying so is signed.
func TestTheRulesHandshakeMustMatch(t *testing.T) {
	_, games, _ := table(t)
	if !games[0].StartAgreed(sid) || !games[1].StartAgreed(sid) {
		t.Fatal("two peers on the same build did not agree their rules")
	}
	if why := games[0].StartProblem(sid); why != "" {
		t.Fatalf("agreement reported a problem: %s", why)
	}
}

// A correctly signed statement from a real seat, naming a different rulebook.
// It must stop the table rather than be played through: two builds that
// disagree about a deadline disagree about who owes money.
func TestADifferentRulebookStopsTheTable(t *testing.T) {
	net, games, _ := table(t)

	other := session.Ours()
	other.MatchDeadlineBlocks = 999
	// The honest key refuses to state a second rulebook at its own position -
	// that is the durable book doing its job. So model what a patched client
	// does instead: the same private key with no memory of what it signed.
	// Doing that publishes its own key, which is the point.
	loose, err := forfeit.LogKeyFrom(net.keys[1].Priv(), matchID)
	if err != nil {
		t.Fatalf("loose key: %v", err)
	}
	body := signAs(t, loose, other)

	err = games[0].Handle(context.Background(), sdk.Message{
		Match: sid, GCID: "gc", From: "peer", Kind: session.KindStart, Body: body,
	})
	if err == nil {
		t.Fatal("a peer playing a different rulebook was accepted")
	}
	if games[0].StartAgreed(sid) {
		t.Fatal("the table still reports agreement")
	}
	if why := games[0].StartProblem(sid); why == "" {
		t.Fatal("the table does not say why it stopped")
	}
	if ok, _ := games[0].Playable(sid); ok {
		t.Fatal("a table whose seats disagree about the rules is playable")
	}
	if err := games[0].Play(context.Background(), sid, 3); err == nil {
		t.Fatal("a move was signed for a table whose seats disagree about the rules")
	}
}

// signAs builds a start message signed by the given seat's own log key.
func signAs(t *testing.T, key *forfeit.LogKey, m manifest.Manifest) []byte {
	t.Helper()
	body, err := session.SignStart(key, matchID, m)
	if err != nil {
		t.Fatalf("sign start: %v", err)
	}
	return body
}

// An abandonment is worth money: it turns a board somebody is losing into a
// void that returns their stake. A signature proves who said it, not that it
// was true, so every part of the claim is recomputed from the log.
func TestAForgedAbandonmentIsRefused(t *testing.T) {
	net, games, roster := table(t)
	// Seat 0 moves, so seat 1 owes the next one and is the only seat that
	// could honestly be called stalled.
	if err := games[0].Play(context.Background(), sid, 3); err != nil {
		t.Fatalf("opening move: %v", err)
	}
	view, _ := games[0].View(sid)
	net.mineTo(int64(view.Deadline) + 5)

	// A mirror of the chain, so the test can build statements that are
	// structurally valid and sign them as seat 1 with no memory of what it
	// already signed - which is what a patched client is.
	blob, err := games[0].Transcript(sid)
	if err != nil {
		t.Fatalf("transcript: %v", err)
	}
	mirror, err := movelog.Unmarshal(blob, roster)
	if err != nil {
		t.Fatalf("mirror: %v", err)
	}
	last, _ := mirror.LastHeight()

	// A fresh unbooked key per attempt: one attacker cannot make two different
	// statements at one position without publishing its key, so each forgery
	// gets its own, and the check under test is the one that refuses it.
	loose := func(seat int) *forfeit.LogKey {
		k, err := forfeit.LogKeyFrom(net.keys[seat].Priv(), matchID)
		if err != nil {
			t.Fatalf("loose key: %v", err)
		}
		return k
	}

	for name, tc := range map[string]struct {
		build  func() (*movelog.Abandon, error)
		target int
	}{
		"names the wrong seat as stalled": {
			// Seat 1 owes the move and blames seat 0 anyway.
			build: func() (*movelog.Abandon, error) {
				return mirror.Abandon(loose(1), 1, 0, movelog.ReasonStall, 0, last+session.MoveDeadlineBlocks)
			},
			target: 0,
		},
		"invents a deadline the log does not imply": {
			build: func() (*movelog.Abandon, error) {
				return mirror.Abandon(loose(0), 0, 1, movelog.ReasonStall, 0, 1)
			},
			target: 1,
		},
		"names a board that is not in play": {
			build: func() (*movelog.Abandon, error) {
				return mirror.Abandon(loose(0), 0, 1, movelog.ReasonStall, 2, last+session.MoveDeadlineBlocks)
			},
			target: 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			a, err := tc.build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			body, err := a.MarshalJSON()
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if err := games[tc.target].Handle(context.Background(), sdk.Message{
				Match: sid, GCID: "gc", From: "peer", Kind: session.KindAbandon, Body: body,
			}); err == nil {
				t.Fatalf("an abandonment that %s was accepted", name)
			}
			chainLoop(t, games)
			if v, _ := games[tc.target].View(sid); v.Abandoned {
				t.Fatalf("an abandonment that %s voided the match", name)
			}
		})
	}
}

// An honest claim that is simply early is held, not thrown away: waiting makes
// it true, and both peers reach the same answer from the same log.
func TestAnEarlyAbandonmentIsHeldUntilTheChainCatchesUp(t *testing.T) {
	net, games, roster := table(t)
	if err := games[0].Play(context.Background(), sid, 3); err != nil {
		t.Fatalf("opening move: %v", err)
	}
	blob, _ := games[0].Transcript(sid)
	mirror, err := movelog.Unmarshal(blob, roster)
	if err != nil {
		t.Fatalf("mirror: %v", err)
	}
	loose, err := forfeit.LogKeyFrom(net.keys[1].Priv(), matchID)
	if err != nil {
		t.Fatalf("loose key: %v", err)
	}
	last, _ := mirror.LastHeight()
	// Seat 1 says seat 1 is stalled - nonsense - so use the honest shape:
	// seat 1 cannot accuse itself, so have seat 1 claim the match expired.
	first, _ := mirror.FirstHeight()
	a, err := mirror.Abandon(loose, 1, 0, movelog.ReasonExpired, 0, first+session.MatchDeadlineBlocks)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	body, _ := a.MarshalJSON()
	if err := games[0].Handle(context.Background(), sdk.Message{
		Match: sid, GCID: "gc", From: "peer", Kind: session.KindAbandon, Body: body,
	}); err != nil {
		t.Fatalf("a well-formed claim was refused outright: %v", err)
	}

	// Before the deadline it is held, and the match is still live.
	net.mineTo(int64(last))
	chainLoop(t, games)
	if v, _ := games[0].View(sid); v.Abandoned {
		t.Fatal("a claim was applied before its deadline")
	}
	// Past it, the same claim takes effect with no new message.
	net.mineTo(int64(first + session.MatchDeadlineBlocks))
	chainLoop(t, games)
	if v, _ := games[0].View(sid); !v.Abandoned {
		t.Fatal("a claim whose deadline passed was never applied")
	}
}

// Both peers must propose the same numbers. The bridge identifies a payout by
// the transaction it builds, so a single atom of difference is two
// transactions, each half-signed, and the pot waits out the refund locks.
func TestBothPeersProposeIdenticalShares(t *testing.T) {
	net, games, _ := table(t)
	for i := 0; i < 3; i++ {
		playBoard(t, games, openerWinsIn7)
	}
	for seat, g := range games {
		if err := g.Settle(context.Background(), sid); err != nil {
			t.Fatalf("seat %d settling: %v", seat, err)
		}
	}
	net.mu.Lock()
	defer net.mu.Unlock()
	if len(net.settled) != 2 {
		t.Fatalf("%d proposals, want one per seat", len(net.settled))
	}
	a, b := net.settled[0], net.settled[1]
	if a.Void != b.Void || len(a.Shares) != len(b.Shares) {
		t.Fatalf("the peers proposed different outcomes: %+v and %+v", a, b)
	}
	for seat, atoms := range a.Shares {
		if b.Shares[seat] != atoms {
			t.Fatalf("seat %d is paid %d by one peer and %d by the other",
				seat, atoms, b.Shares[seat])
		}
	}
}

// Nothing is co-signed until this peer has itself reached a settled outcome.
func TestCoSigningIsWithheldUntilThisPeerHasItsOwnResult(t *testing.T) {
	_, games, _ := table(t)
	if games[0].WillCoSign(sid, 0) {
		t.Fatal("a peer agreed to co-sign a payout before the match was played")
	}
	for i := 0; i < 3; i++ {
		playBoard(t, games, openerWinsIn7)
	}
	if games[0].WillCoSign(sid, 0) {
		t.Fatal("a peer agreed to co-sign before it had proposed anything itself")
	}
	if err := games[0].Settle(context.Background(), sid); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if !games[0].WillCoSign(sid, 0) {
		t.Fatal("a peer that settled its own verified outcome refuses to co-sign it")
	}
}

// A process that dies mid-match must come back knowing what it played. Without
// this the seats cannot carry on and cannot settle, and both stakes wait out
// their refund locks for a crash.
func TestAMatchSurvivesARestart(t *testing.T) {
	net, games, seatKeys := tableWithDirs(t)
	playBoard(t, games, openerWinsIn7)
	// Seat 1 opens the second board, so ask whoever is to move.
	opener, _ := games[0].View(sid)
	if err := games[opener.Turn].Play(context.Background(), sid, 2); err != nil {
		t.Fatalf("a move on the second board: %v", err)
	}
	before, _ := games[0].View(sid)
	if before.Moves == 0 {
		t.Fatal("nothing was played")
	}
	blob, err := games[0].Transcript(sid)
	if err != nil {
		t.Fatalf("transcript: %v", err)
	}
	if err := games[0].Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// A fresh game over the same profile directory, reopened the way a driver
	// does after a restart: from what is on disk, not from the Seated hook.
	revived, err := session.New(seatKeys.dirs[0])
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	t.Cleanup(func() { revived.Close() })
	revived.Bind(&stub{t: t, seat: 0, key: seatKeys.keys[0], seats: seatKeys.seats, net: net})
	if err := revived.Ensure(sid); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	after, ok := revived.View(sid)
	if !ok {
		t.Fatal("the table did not come back")
	}
	if after.Moves != before.Moves || after.Head != before.Head {
		t.Fatalf("it came back at move %d/%s, was %d/%s",
			after.Moves, after.Head[:8], before.Moves, before.Head[:8])
	}
	if after.Board != before.Board || after.Turn != before.Turn || after.Score != before.Score {
		t.Fatalf("it came back on board %d, seat %d to play, score %v; was %d/%d/%v",
			after.Board, after.Turn, after.Score, before.Board, before.Turn, before.Score)
	}
	again, err := revived.Transcript(sid)
	if err != nil {
		t.Fatalf("transcript: %v", err)
	}
	if string(again) != string(blob) {
		t.Fatal("the revived table holds a different history")
	}
}
