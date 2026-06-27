package xwblog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// goldenFixture is the committed real-CPRS-login capture (see testdata/README.md):
// 329 names-only level-2 records of a sign-on + chart browse carried through the
// relay. It is the regression corpus for the parse → classify → LDJSON pipeline
// and the on-disk envelope contract.
const goldenFixture = "../../testdata/cprs-login.ldjson"

// wireRecord mirrors Record.LDJSON()'s on-disk envelope, for reading the fixture.
type wireRecord struct {
	Source        string `json:"source"`
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	RPC           string `json:"rpc"`
	TS            string `json:"ts"`
	Job           int    `json:"job"`
	Seq           int    `json:"seq"`
	Msg           string `json:"msg"`
}

func readGolden(t *testing.T) []string {
	t.Helper()
	f, err := os.Open(filepath.Clean(goldenFixture))
	if err != nil {
		t.Fatalf("open golden fixture: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan golden fixture: %v", err)
	}
	return lines
}

// TestGoldenRoundTrips is the heart of the golden test: for every record in the
// real capture, reconstruct the raw broker log node value ("$H^message") it came
// from, run it back through ParseRecord, and assert the re-rendered LDJSON is
// byte-identical to the committed line. This locks the whole
// ParseRecord→classify→LDJSON pipeline and the on-disk envelope against drift,
// using a real 329-record corpus rather than synthetic input.
func TestGoldenRoundTrips(t *testing.T) {
	lines := readGolden(t)
	if len(lines) == 0 {
		t.Fatal("golden fixture is empty")
	}
	for i, line := range lines {
		var w wireRecord
		if err := json.Unmarshal([]byte(line), &w); err != nil {
			t.Fatalf("line %d: not valid JSON: %v", i+1, err)
		}
		// Rebuild the raw node value and subscript the parser would have seen.
		value := w.TS + "^" + w.Msg
		job := "XWBLOG" + strconv.Itoa(w.Job)
		got := ParseRecord(job, w.Seq, value).LDJSON()
		if got != line {
			t.Fatalf("line %d round-trip drift:\n  fixture: %s\n  parsed : %s", i+1, line, got)
		}
	}
}

// TestGoldenEnvelopeInvariants asserts the committed fixture stays a valid,
// PHI-free, names-only level-2 capture — a guard on the data itself, not just
// the parser.
func TestGoldenEnvelopeInvariants(t *testing.T) {
	lines := readGolden(t)
	const wantCount = 329
	if len(lines) != wantCount {
		t.Errorf("fixture has %d records, expected %d (regenerated? update the count)", len(lines), wantCount)
	}
	for i, line := range lines {
		var w wireRecord
		if err := json.Unmarshal([]byte(line), &w); err != nil {
			t.Fatalf("line %d: %v", i+1, err)
		}
		if w.Source != "xwbdebug" || w.SchemaVersion != 1 {
			t.Errorf("line %d: envelope drift: source=%q schema_version=%d", i+1, w.Source, w.SchemaVersion)
		}
		// PHI guard: every record is a bare RPC name (level 2) — kind rpc, and the
		// message is exactly "RPC: <name>", never a parameter string (level 3).
		if w.Kind != string(KindRPC) {
			t.Errorf("line %d: kind=%q, want rpc (a non-names line leaked into the fixture)", i+1, w.Kind)
		}
		if w.Msg != "RPC: "+w.RPC {
			t.Errorf("line %d: msg %q is not a bare name for rpc %q — possible level-3/PHI leak", i+1, w.Msg, w.RPC)
		}
	}
}

// TestGoldenCanonicalSignon pins the opening sign-on RPC sequence — the
// documented CPRS handshake — so a regenerated fixture that no longer starts
// with a real login is caught.
func TestGoldenCanonicalSignon(t *testing.T) {
	lines := readGolden(t)
	var names []string
	for _, line := range lines {
		var w wireRecord
		if err := json.Unmarshal([]byte(line), &w); err != nil {
			t.Fatalf("bad line: %v", err)
		}
		names = append(names, w.RPC)
	}
	wantPrefix := []string{
		"XUS SIGNON SETUP",
		"XUS INTRO MSG",
		"XUS AV CODE",
		"XUS GET USER INFO",
		"XWB GET BROKER INFO",
	}
	if len(names) < len(wantPrefix) {
		t.Fatalf("fixture too short (%d records) to hold the signon", len(names))
	}
	for i, want := range wantPrefix {
		if names[i] != want {
			t.Errorf("signon RPC %d = %q, want %q\n(full opening: %s)",
				i, names[i], want, strings.Join(names[:len(wantPrefix)], " → "))
		}
	}
}
