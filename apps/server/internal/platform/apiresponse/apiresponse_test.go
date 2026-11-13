package apiresponse

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

func TestError(t *testing.T) {
	assert.Equal(t, api.Error{Code: "x", Message: "went wrong"}, Error("x", errors.New("went wrong")))
}

func TestValidation(t *testing.T) {
	response := Validation(&validation.Error{Fields: []validation.FieldError{{Field: "email", Message: "is invalid"}}})

	assert.Equal(t, "validation_failed", response.Code)
	assert.Equal(t, []api.FieldError{{Field: "email", Message: "is invalid"}}, *response.Fields)
}

func TestSession(t *testing.T) {
	when := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	found := sessionstore.Session{ID: uuid.New(), DeviceName: "Laptop", CreatedAt: when, LastSeenAt: when, ExpiresAt: when}

	assert.True(t, Session(found, found.ID).Current)
	assert.False(t, Session(found, uuid.New()).Current)
	assert.Equal(t, "Laptop", Session(found, found.ID).DeviceName)
}

func TestDeviceName(t *testing.T) {
	assert.Equal(t, "Unknown device", DeviceName("   "))
	assert.Equal(t, "Unknown device", DeviceName("\xff"))
	assert.Equal(t, "Firefox on Linux", DeviceName("Firefox\ton  Linux"))
	assert.Equal(t, strings.Repeat("é", 64), DeviceName(strings.Repeat("é", 70)))
}

func TestAvatarPath(t *testing.T) {
	assert.Nil(t, AvatarPath(nil))
	id := uuid.MustParse("01890000-0000-7000-8000-000000000001")
	assert.Equal(t, "/api/v1/avatars/"+id.String(), *AvatarPath(&id))
}
