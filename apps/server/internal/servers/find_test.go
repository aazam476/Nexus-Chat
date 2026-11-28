package servers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
)

func TestFindingServersChannelsAndPeopleByName(t *testing.T) {
	f := newFixture(t)
	ada, grace, stranger := f.user(t, "ada"), f.user(t, "grace"), f.user(t, "gracie")
	details := f.server(t, ada, grace)
	f.server(t, stranger)
	owner := Scope{details.Server.ID, ada}
	general := channelNamed(t, details, "general")

	found, err := f.service.Find(t.Context(), grace, "  BOOK ")
	require.NoError(t, err)
	require.Len(t, found.Servers, 1, "only servers the user is in, whatever the case")
	assert.Equal(t, details.Server.ID, found.Servers[0].ID)

	found, err = f.service.Find(t.Context(), grace, "gen")
	require.NoError(t, err)
	require.Len(t, found.Channels, 2, "the text and voice channels")
	assert.Contains(t, found.Channels, FoundChannel{ID: general.ID, ServerID: details.Server.ID, Kind: KindText, Name: "general", ServerName: "Book club"})

	// A channel the user cannot see is not found.
	require.NoError(t, f.service.SetOverride(t.Context(), owner, general.ID, Override{TargetKind: TargetMember, TargetID: grace, Deny: permissions.ViewChannel}))
	found, err = f.service.Find(t.Context(), grace, "gen")
	require.NoError(t, err)
	assert.NotContains(t, found.Channels, FoundChannel{ID: general.ID, ServerID: details.Server.ID, Kind: KindText, Name: "general", ServerName: "Book club"})
	assert.Len(t, found.Channels, 1)
	found, err = f.service.Find(t.Context(), ada, "gen")
	require.NoError(t, err)
	assert.Len(t, found.Channels, 2)

	// People who share a server, never the user or strangers.
	found, err = f.service.Find(t.Context(), ada, "gra")
	require.NoError(t, err)
	assert.Equal(t, []FoundPerson{{ID: grace, Username: "grace", DisplayName: "grace"}}, found.People)
	found, err = f.service.Find(t.Context(), grace, "ada")
	require.NoError(t, err)
	assert.Equal(t, []FoundPerson{{ID: ada, Username: "ada", DisplayName: "ada"}}, found.People)
}

func TestFindingTakesWildcardsLiterally(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	f.server(t, ada)
	for _, query := range []string{"%%", "__", `\\`, "b%k"} {
		found, err := f.service.Find(t.Context(), ada, query)
		require.NoError(t, err)
		assert.Empty(t, found.Servers, query)
		assert.Empty(t, found.Channels, query)
	}
	assert.Equal(t, `%50\% off\_now\\%`, pattern(`50% off_now\`))
}

func TestFindingNeedsSensibleWords(t *testing.T) {
	f := newFixture(t)
	ada := f.user(t, "ada")
	for _, query := range []string{"", " a ", string(make([]rune, 101))} {
		_, err := f.service.Find(t.Context(), ada, query)
		require.ErrorIs(t, err, ErrFindQuery)
	}
}

func TestTheSearchHandlerAnswersTheContract(t *testing.T) {
	f := newFixture(t)
	ada, grace := f.user(t, "ada"), f.user(t, "grace")
	details := f.server(t, ada, grace)
	handler := NewHandler(f.service)

	response, err := handler.SearchNames(as(t, ada), api.SearchNamesRequestObject{Params: api.SearchNamesParams{Q: "e"}})
	require.NoError(t, err)
	require.IsType(t, api.SearchNames422JSONResponse{}, response)

	response, err = handler.SearchNames(as(t, ada), api.SearchNamesRequestObject{Params: api.SearchNamesParams{Q: "ra"}})
	require.NoError(t, err)
	found, ok := response.(api.SearchNames200JSONResponse)
	require.True(t, ok)
	assert.Equal(t, []api.FoundPerson{{Id: grace, Username: "grace", DisplayName: "grace"}}, found.People)
	assert.Equal(t, []api.FoundServer{}, found.Servers)

	response, err = handler.SearchNames(as(t, grace), api.SearchNamesRequestObject{Params: api.SearchNamesParams{Q: "general"}})
	require.NoError(t, err)
	found, ok = response.(api.SearchNames200JSONResponse)
	require.True(t, ok)
	require.Len(t, found.Channels, 2)
	assert.Equal(t, details.Server.ID, found.Channels[0].ServerId)
	response, err = handler.SearchNames(as(t, grace), api.SearchNamesRequestObject{Params: api.SearchNamesParams{Q: "club"}})
	require.NoError(t, err)
	assert.Equal(t, details.Server.Name, response.(api.SearchNames200JSONResponse).Servers[0].Name)
}
