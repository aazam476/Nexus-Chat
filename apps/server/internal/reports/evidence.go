package reports

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/google/uuid"
)

// The evidence's layout is the crypto package's encodeEvidence: eight
// length-prefixed fields, each a 4-byte big-endian length and the bytes.
// The server reads only the address and epoch, to check the evidence is
// about the reported message; moderators' devices check the signature.
const (
	evidenceLabel  = "nexus-chat archive v1"
	evidenceFields = 8
	// MaximumEvidence is how large evidence may be: a message's largest size.
	MaximumEvidence = 4 << 20
)

// ErrBadEvidence means the evidence is not about the reported message.
var ErrBadEvidence = errors.New("the evidence is not for this message")

// evidenceAbout is what evidence must name: where the archive copy is
// addressed (the channel's ID, or the conversation's), the message and its epoch.
type evidenceAbout struct {
	place   uuid.UUID
	message uuid.UUID
	epoch   int64
}

// fields splits length-prefixed bytes into exactly count fields.
func fields(data []byte, count int) ([][]byte, bool) {
	out := make([][]byte, 0, count)
	for len(out) < count {
		if len(data) < 4 {
			return nil, false
		}
		size := int64(binary.BigEndian.Uint32(data))
		if size > int64(len(data)-4) {
			return nil, false
		}
		out = append(out, data[4:4+size])
		data = data[4+size:]
	}
	return out, len(data) == 0
}

// checkEvidence checks that evidence is well formed and names the message.
func checkEvidence(evidence []byte, about evidenceAbout) error {
	if len(evidence) > MaximumEvidence {
		return ErrBadEvidence
	}
	parts, ok := fields(evidence, evidenceFields)
	if !ok || string(parts[0]) != evidenceLabel || len(parts[3]) != 8 {
		return ErrBadEvidence
	}
	epoch := binary.BigEndian.Uint64(parts[3])
	if !bytes.Equal(parts[1], []byte(about.place.String())) || !bytes.Equal(parts[2], []byte(about.message.String())) ||
		about.epoch < 0 || epoch != uint64(about.epoch) {
		return ErrBadEvidence
	}
	return nil
}
