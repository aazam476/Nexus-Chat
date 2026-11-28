package servers

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// SearchNames answers GET /api/v1/search.
func (handler *Handler) SearchNames(ctx context.Context, request api.SearchNamesRequestObject) (api.SearchNamesResponseObject, error) {
	me, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	found, err := handler.service.Find(ctx, me, request.Params.Q)
	if errors.Is(err, ErrFindQuery) {
		return api.SearchNames422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(apiresponse.Error("invalid", err))}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.SearchNames200JSONResponse(foundResponse(found)), nil
}

func foundResponse(found Found) api.NameSearchResults {
	out := api.NameSearchResults{
		Servers:  make([]api.FoundServer, len(found.Servers)),
		Channels: make([]api.FoundChannel, len(found.Channels)),
		People:   make([]api.FoundPerson, len(found.People)),
	}
	for index, server := range found.Servers {
		out.Servers[index] = api.FoundServer{Id: server.ID, Name: server.Name, IconId: server.IconID}
	}
	for index, channel := range found.Channels {
		out.Channels[index] = api.FoundChannel{Id: channel.ID, ServerId: channel.ServerID, ServerName: channel.ServerName, Kind: api.FoundChannelKind(channel.Kind), Name: channel.Name}
	}
	for index, person := range found.People {
		out.People[index] = api.FoundPerson{Id: person.ID, Username: person.Username, DisplayName: person.DisplayName}
	}
	return out
}
