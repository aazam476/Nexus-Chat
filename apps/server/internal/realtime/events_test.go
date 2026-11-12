package realtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelationshipChangesReachTheUsersDevices(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	phone := fixture.dial(t)
	phone.identify(aliceToken)
	other := uuid.New()

	fixture.hub.RelationshipChanged(t.Context(), fixture.user.ID, other, "incoming")
	fixture.hub.RelationshipChanged(t.Context(), uuid.New(), other, "friend") // nobody connected: nothing happens

	frame := phone.receiveType(TypeRelationshipUpdate)
	var data relationshipData
	require.NoError(t, json.Unmarshal(frame.Data, &data))
	assert.Equal(t, relationshipData{UserID: other, Relationship: "incoming"}, data)
	assert.Positive(t, frame.Seq, "an event, so it is replayed on resume")
}

func TestReadChangesReachTheUsersOwnDevices(t *testing.T) {
	fixture := newGatewayFixture(t, time.Second)
	laptop := fixture.dial(t)
	laptop.identify(aliceToken)
	conversation := uuid.New()

	fixture.hub.ReadChanged(t.Context(), fixture.user.ID, conversation, 42)

	frame := laptop.receiveType(TypeReadUpdate)
	var data readData
	require.NoError(t, json.Unmarshal(frame.Data, &data))
	assert.Equal(t, readData{ConversationID: conversation, LastReadID: 42}, data)
}
