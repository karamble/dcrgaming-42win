package session_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/karamble/dcrgaming-42win/internal/audit"
	"github.com/karamble/dcrgaming-42win/internal/movelog"
	"github.com/karamble/dcrgaming-42win/internal/session"
)

func TestReceiptExportsAuditableMatchWithoutSecrets(t *testing.T) {
	_, games, roster := table(t)
	for i := 0; i < 3; i++ {
		playBoard(t, games, openerWinsIn7)
	}
	r, err := games[0].Receipt(sid)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Complete || r.Match != matchID || r.Table != sid || len(r.Results) != 3 {
		t.Fatal("missing match metadata")
	}
	m, err := audit.Match(r.Transcript, roster, 0)
	if err != nil || !m.Done() {
		t.Fatal("export cannot be independently verified", err)
	}
	dir := t.TempDir()
	path, err := r.Export(dir)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatal("export is not owner-only")
	}
	path2, err := r.Export(dir)
	if err != nil || path == path2 {
		t.Fatal("export overwrote prior receipt")
	}
	bytes, _ := os.ReadFile(path)
	for _, secret := range []string{"PRIVATE KEY", "client_private_key", "seed", "nonce"} {
		if strings.Contains(string(bytes), secret) {
			t.Fatal("sensitive data in receipt", secret)
		}
	}
	var decoded session.Receipt
	if err = json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err = json.Unmarshal(decoded.Transcript, &raw); err != nil {
		t.Fatal(err)
	}
	raw["entries"].([]any)[0].(map[string]any)["column"] = float64(6)
	tampered, _ := json.Marshal(raw)
	if _, err = audit.Match(tampered, roster, 0); err == nil {
		t.Fatal("tampered move accepted")
	}
	if _, err = r.Export(path); err == nil {
		t.Fatal("file accepted as export directory")
	}
}

func TestIncompleteAndAbandonedReceiptKeepSeparateEvidence(t *testing.T) {
	net, games, roster := table(t)
	playBoard(t, games, []uint8{3})
	r, err := games[0].Receipt(sid)
	if err != nil {
		t.Fatal(err)
	}
	if r.Complete || len(r.Abandonment) != 0 {
		t.Fatal("unfinished match marked complete")
	}
	net.mineTo(900 + int64(session.MoveDeadlineBlocks))
	if err = games[0].Abandon(context.Background(), sid); err != nil {
		t.Fatal(err)
	}
	r, err = games[0].Receipt(sid)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Complete || len(r.Abandonment) == 0 {
		t.Fatal("missing abandonment evidence")
	}
	a, err := movelog.DecodeAbandon(r.Abandonment)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Verify(); err != nil {
		t.Fatal(err)
	}
	m, err := audit.Match(r.Transcript, roster, 0)
	if err != nil || m.Done() {
		t.Fatal("abandonment incorrectly converted to completed move log")
	}
}
