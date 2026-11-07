// Package apiresponse converts service results into the generated API
// types where several features need the same conversion, so every endpoint
// reports errors, sessions and device names the same way.
package apiresponse

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

const (
	maximumDeviceNameLength = 64
	unknownDevice           = "Unknown device"
)

// Error builds an API error with a stable code and err's message.
func Error(code string, err error) api.Error {
	return api.Error{Code: code, Message: err.Error()}
}

// Validation lists every invalid field.
func Validation(problems *validation.Error) api.Error {
	fields := make([]api.FieldError, 0, len(problems.Fields))
	for _, field := range problems.Fields {
		fields = append(fields, api.FieldError{Field: field.Field, Message: field.Message})
	}
	return api.Error{Code: "validation_failed", Message: "Some fields are invalid.", Fields: &fields}
}

// Session describes a session; current marks the one making the request.
func Session(found sessionstore.Session, currentSessionID uuid.UUID) api.Session {
	return api.Session{
		Id: found.ID, DeviceName: found.DeviceName, CreatedAt: found.CreatedAt,
		LastSeenAt: found.LastSeenAt, ExpiresAt: found.ExpiresAt, Current: found.ID == currentSessionID,
	}
}

// DeviceName tidies a client-supplied device name: whitespace collapsed,
// at most 64 characters, "Unknown device" if empty. A bad device name is
// not worth refusing a login over.
func DeviceName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || !utf8.ValidString(name) {
		return unknownDevice
	}
	if utf8.RuneCountInString(name) > maximumDeviceNameLength {
		name = string([]rune(name)[:maximumDeviceNameLength])
	}
	return name
}
