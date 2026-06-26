package xwbwire

import "testing"

func TestRPCMessage(t *testing.T) {
	// Golden bytes for a no-arg RPC, per the XWB broker wire protocol:
	// [XWB] + 4-byte header "0030" + chunk type '2' (RPC) + SREAD(ver) +
	// SREAD(name) + EOT. SREAD = one length byte + value.
	got := RPCMessage("XWB IM HERE")
	want := []byte("[XWB]00302")
	want = append(want, 0x01, '0')                // SREAD("0")  -> len 1, "0"
	want = append(want, 0x0b)                     // SREAD len = 11
	want = append(want, []byte("XWB IM HERE")...) // the name
	want = append(want, 0x04)                     // EOT
	if string(got) != string(want) {
		t.Fatalf("RPCMessage mismatch:\n got %v\nwant %v", got, want)
	}
}

func TestRPCMessageShortName(t *testing.T) {
	got := RPCMessage("X")
	want := []byte("[XWB]00302")   // [XWB] + header "0030" + chunk '2'
	want = append(want, 0x01, '0') // SREAD("0")
	want = append(want, 0x01, 'X') // SREAD("X")
	want = append(want, 0x04)      // EOT
	if string(got) != string(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
