// Package mlsheader reads the few cleartext fields of an MLS message that
// the delivery service needs (RFC 9420 §6; ADR 0002): the wire format,
// group ID, epoch and content type, and for a PublicMessage its sender.
// It does not verify signatures or decrypt anything; the server runs no
// MLS, and members reject what is not valid. Every input is untrusted, so
// the parser is fuzz-tested.
package mlsheader

import (
	"encoding/binary"
	"errors"
)

// Wire formats (RFC 9420 §6).
const (
	WireFormatPublicMessage  = 1
	WireFormatPrivateMessage = 2
	WireFormatWelcome        = 3
	WireFormatGroupInfo      = 4
	WireFormatKeyPackage     = 5
)

// Content types (RFC 9420 §6).
const (
	ContentTypeApplication = 1
	ContentTypeProposal    = 2
	ContentTypeCommit      = 3
)

// Sender types of a PublicMessage (RFC 9420 §6).
const (
	SenderMember            = 1
	SenderExternal          = 2
	SenderNewMemberProposal = 3
	SenderNewMemberCommit   = 4
)

const protocolVersionMLS10 = 1

// ErrMalformed means the bytes are not an MLS message this parser reads.
var ErrMalformed = errors.New("not a well-formed MLS 1.0 group message")

// Header is what the delivery service reads from a group message.
type Header struct {
	WireFormat  uint16
	GroupID     []byte
	Epoch       uint64
	ContentType uint8
	// SenderType is set for PublicMessage only (PrivateMessage hides it).
	SenderType uint8
}

// reader walks a byte slice; any read past the end makes it fail.
type reader struct {
	data   []byte
	failed bool
}

func (r *reader) take(count int) []byte {
	if r.failed || count < 0 || count > len(r.data) {
		r.failed = true
		return nil
	}
	taken := r.data[:count]
	r.data = r.data[count:]
	return taken
}

func (r *reader) uint8() uint8 {
	if bytes := r.take(1); bytes != nil {
		return bytes[0]
	}
	return 0
}

func (r *reader) uint16() uint16 {
	if bytes := r.take(2); bytes != nil {
		return binary.BigEndian.Uint16(bytes)
	}
	return 0
}

func (r *reader) uint32() uint32 {
	if bytes := r.take(4); bytes != nil {
		return binary.BigEndian.Uint32(bytes)
	}
	return 0
}

func (r *reader) uint64() uint64 {
	if bytes := r.take(8); bytes != nil {
		return binary.BigEndian.Uint64(bytes)
	}
	return 0
}

// variable reads an opaque<V>: a QUIC-style variable-length length in its
// shortest form, then that many bytes (RFC 9420 §2.1.2).
func (r *reader) variable() []byte {
	first := r.uint8()
	if r.failed {
		return nil
	}
	var length uint64
	switch first >> 6 {
	case 0:
		length = uint64(first & 0x3f)
	case 1:
		length = uint64(first&0x3f)<<8 | uint64(r.uint8())
		if length < 64 {
			r.failed = true
		}
	case 2:
		rest := r.take(3)
		if rest == nil {
			return nil
		}
		length = uint64(first&0x3f)<<24 | uint64(rest[0])<<16 | uint64(rest[1])<<8 | uint64(rest[2])
		if length < 16384 {
			r.failed = true
		}
	default:
		r.failed = true
	}
	if r.failed || length > uint64(len(r.data)) {
		r.failed = true
		return nil
	}
	return r.take(int(length))
}

// Parse reads the header of a PublicMessage or PrivateMessage.
func Parse(message []byte) (Header, error) {
	r := &reader{data: message}
	if r.uint16() != protocolVersionMLS10 {
		return Header{}, ErrMalformed
	}
	header := Header{WireFormat: r.uint16()}
	header.GroupID = r.variable()
	header.Epoch = r.uint64()
	switch header.WireFormat {
	case WireFormatPublicMessage:
		header.SenderType = r.uint8()
		switch header.SenderType {
		case SenderMember, SenderExternal:
			r.uint32() // leaf or sender index
		case SenderNewMemberProposal, SenderNewMemberCommit:
		default:
			return Header{}, ErrMalformed
		}
		r.variable() // authenticated data
		header.ContentType = r.uint8()
	case WireFormatPrivateMessage:
		header.ContentType = r.uint8()
	default:
		return Header{}, ErrMalformed
	}
	if r.failed || header.ContentType < ContentTypeApplication || header.ContentType > ContentTypeCommit {
		return Header{}, ErrMalformed
	}
	return header, nil
}

// IsWireFormat reports whether message is an MLS 1.0 message of the given wire format.
func IsWireFormat(message []byte, wireFormat uint16) bool {
	r := &reader{data: message}
	return r.uint16() == protocolVersionMLS10 && r.uint16() == wireFormat && !r.failed && len(r.data) > 0
}
