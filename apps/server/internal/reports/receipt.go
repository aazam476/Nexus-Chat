package reports

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// receiptLabel separates receipt signatures from anything else the
// instance key signs.
const receiptLabel = "nexus-chat report receipt v1\n"

// Receipt is the instance's statement about a stored message: it holds a
// copy with this hash, sent by this user to this conversation (and
// channel) at this time.
type Receipt struct {
	ConversationID uuid.UUID  `json:"conversationId"`
	ChannelID      *uuid.UUID `json:"channelId,omitempty"`
	MessageID      uuid.UUID  `json:"messageId"`
	SenderID       uuid.UUID  `json:"senderId"`
	// Sender is the sender's address, @name@domain, which their device
	// credential names.
	Sender string `json:"sender"`
	Epoch  int64  `json:"epoch"`
	// ArchiveHash is the SHA-256 of the archive copy as stored.
	ArchiveHash []byte    `json:"archiveHash"`
	SentAt      time.Time `json:"sentAt"`
	IssuedAt    time.Time `json:"issuedAt"`
}

// SignedReceipt is a receipt's exact JSON and the signature over the label and it.
type SignedReceipt struct {
	Receipt   []byte
	Signature []byte
}

// SignReceipt signs receipt with key.
func SignReceipt(key ed25519.PrivateKey, receipt Receipt) (SignedReceipt, error) {
	payload, err := json.Marshal(receipt)
	if err != nil {
		return SignedReceipt{}, fmt.Errorf("encoding the receipt: %w", err)
	}
	return SignedReceipt{Receipt: payload, Signature: ed25519.Sign(key, append([]byte(receiptLabel), payload...))}, nil
}

// VerifyReceipt checks a receipt against the instance's public key.
func VerifyReceipt(key ed25519.PublicKey, signed SignedReceipt) bool {
	return ed25519.Verify(key, append([]byte(receiptLabel), signed.Receipt...), signed.Signature)
}

// archiveHash is what a receipt says about an archive copy.
func archiveHash(sealed []byte) []byte {
	sum := sha256.Sum256(sealed)
	return sum[:]
}
