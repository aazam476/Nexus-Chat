package mlsheader

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The start of real messages made by packages/crypto (ts-mls): a commit
// that adds a member at epoch 0, and an application message at epoch 1, in
// the group whose ID is sixteen 0xab bytes. The parser reads only the
// beginning, so the rest is left out.
const (
	realCommit  = "0001000210abababababababababababababababab000000000000000003001ceccaa6514283c07bd79cb1dc2f5e21a46ff5731bda10d21bf040bf15"
	realMessage = "0001000210abababababababababababababababab000000000000000101001cfca19b02941d6f9336c9a0f249c6990d8fb98b9cf3a05037ce518fed"
	realWelcome = "00010003000140762098dfd6b1dacfd1949c4cfbb05f01ae817cdb2978c3eea2ce6623ca357147af"
)

func decode(t *testing.T, value string) []byte {
	t.Helper()
	data, err := hex.DecodeString(value)
	require.NoError(t, err)
	return data
}

func TestRealMessagesFromTheClient(t *testing.T) {
	groupID := bytes.Repeat([]byte{0xab}, 16)

	commit, err := Parse(decode(t, realCommit))
	require.NoError(t, err)
	assert.Equal(t, Header{WireFormat: WireFormatPrivateMessage, GroupID: groupID, Epoch: 0, ContentType: ContentTypeCommit}, commit)
	message, err := Parse(decode(t, realMessage))
	require.NoError(t, err)
	assert.Equal(t, Header{WireFormat: WireFormatPrivateMessage, GroupID: groupID, Epoch: 1, ContentType: ContentTypeApplication}, message)
	_, err = Parse(decode(t, realWelcome))
	require.ErrorIs(t, err, ErrMalformed, "a welcome is not a group message")
	assert.True(t, IsWireFormat(decode(t, realWelcome), WireFormatWelcome))
	assert.False(t, IsWireFormat(decode(t, realCommit), WireFormatWelcome))
	assert.False(t, IsWireFormat([]byte{0, 1, 0, 3}, WireFormatWelcome), "nothing after the header")
}

// public builds a PublicMessage header.
func public(senderType byte, rest ...byte) []byte {
	message := []byte{0, 1, 0, 1, 2, 0xaa, 0xbb, 0, 0, 0, 0, 0, 0, 0, 7, senderType}
	return append(message, rest...)
}

func TestPublicMessages(t *testing.T) {
	member, err := Parse(public(SenderMember, 0, 0, 0, 3, 0, ContentTypeCommit))
	require.NoError(t, err)
	assert.Equal(t, Header{WireFormat: WireFormatPublicMessage, GroupID: []byte{0xaa, 0xbb}, Epoch: 7, ContentType: ContentTypeCommit, SenderType: SenderMember}, member)
	joiner, err := Parse(public(SenderNewMemberCommit, 1, 9, ContentTypeCommit))
	require.NoError(t, err)
	assert.Equal(t, uint8(SenderNewMemberCommit), joiner.SenderType)

	for name, message := range map[string][]byte{
		"unknown sender":       public(9, 0, ContentTypeCommit),
		"unknown content type": public(SenderNewMemberCommit, 0, 4),
		"cut off":              public(SenderMember, 0, 0),
	} {
		_, err := Parse(message)
		require.ErrorIs(t, err, ErrMalformed, name)
	}
}

func TestVariableLengthsMustBeShortestAndPresent(t *testing.T) {
	prefix := []byte{0, 1, 0, 2}
	epochAndType := []byte{0, 0, 0, 0, 0, 0, 0, 1, ContentTypeApplication}
	twoByte := append(append(append([]byte{}, prefix...), 0x40, 64), append(bytes.Repeat([]byte{1}, 64), epochAndType...)...)
	header, err := Parse(twoByte)
	require.NoError(t, err)
	assert.Len(t, header.GroupID, 64)
	fourByte := append(append(append([]byte{}, prefix...), 0x80, 0, 0x40, 0), append(bytes.Repeat([]byte{1}, 16384), epochAndType...)...)
	header, err = Parse(fourByte)
	require.NoError(t, err)
	assert.Len(t, header.GroupID, 16384)

	for name, message := range map[string][]byte{
		"two bytes for a short length":  append(append([]byte{}, prefix...), 0x40, 1, 0xaa),
		"four bytes for a short length": append(append([]byte{}, prefix...), 0x80, 0, 0, 1, 0xaa),
		"reserved length prefix":        append(append([]byte{}, prefix...), 0xc0, 0, 0, 0, 0, 0, 0, 1),
		"length past the end":           append(append([]byte{}, prefix...), 5, 1),
		"four-byte length cut off":      append(append([]byte{}, prefix...), 0x80, 0),
		"no group ID at all":            prefix,
		"not MLS 1.0":                   {0, 2, 0, 2, 0},
		"a key package":                 {0, 1, 0, 5, 0},
		"empty":                         {},
	} {
		_, err := Parse(message)
		require.ErrorIs(t, err, ErrMalformed, name)
	}
}
