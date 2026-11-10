package mlsheader

import (
	"encoding/hex"
	"testing"
)

// FuzzParse feeds the parser arbitrary bytes: it must never panic, and
// what it accepts must be a group message of a known content type.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{realCommit, realMessage, realWelcome} {
		data, err := hex.DecodeString(seed)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte{0, 1, 0, 1, 0x40, 64})
	f.Fuzz(func(t *testing.T, message []byte) {
		header, err := Parse(message)
		if err != nil {
			return
		}
		if header.WireFormat != WireFormatPublicMessage && header.WireFormat != WireFormatPrivateMessage {
			t.Fatalf("accepted wire format %d", header.WireFormat)
		}
		if header.ContentType < ContentTypeApplication || header.ContentType > ContentTypeCommit {
			t.Fatalf("accepted content type %d", header.ContentType)
		}
	})
}
