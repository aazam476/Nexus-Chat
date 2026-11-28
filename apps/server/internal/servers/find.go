package servers

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// Server-side search (spec §4.6) covers only what the server holds in
// plain form: the names of the user's servers, the channels they can see
// in them, and the people they share a server with. Messages are searched
// on devices.

// Limits on a search's words.
const (
	findMinimumRunes = 2
	findMaximumRunes = 100
)

// ErrFindQuery means the words are too short or too long to search for.
var ErrFindQuery = fmt.Errorf("search for %d to %d characters", findMinimumRunes, findMaximumRunes)

// FoundServer is a server whose name matches.
type FoundServer struct {
	ID     uuid.UUID
	Name   string
	IconID *uuid.UUID
}

// FoundChannel is a channel the user can see whose name matches.
type FoundChannel struct {
	ID         uuid.UUID
	ServerID   uuid.UUID
	Kind       string
	Name       string
	ServerName string
}

// FoundPerson is someone who shares a server with the user.
type FoundPerson struct {
	ID          uuid.UUID
	Username    string
	DisplayName string
}

// Found is what a search matched.
type Found struct {
	Servers  []FoundServer
	Channels []FoundChannel
	People   []FoundPerson
}

// pattern is an ILIKE pattern for names containing query, with its own
// wildcards escaped.
func pattern(query string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query)
	return "%" + escaped + "%"
}

// Find searches the names userID can see for query.
func (service *Service) Find(ctx context.Context, userID uuid.UUID, query string) (Found, error) {
	query = strings.TrimSpace(query)
	if count := utf8.RuneCountInString(query); count < findMinimumRunes || count > findMaximumRunes {
		return Found{}, ErrFindQuery
	}
	like := pattern(query)
	servers, err := service.store.FindServers(ctx, store.FindServersParams{UserID: userID, Pattern: like})
	if err != nil {
		return Found{}, fmt.Errorf("finding servers: %w", err)
	}
	channels, err := service.visibleChannels(ctx, userID, like)
	if err != nil {
		return Found{}, err
	}
	people, err := service.store.FindPeople(ctx, store.FindPeopleParams{UserID: userID, Pattern: like})
	if err != nil {
		return Found{}, fmt.Errorf("finding people: %w", err)
	}
	found := Found{Channels: channels, Servers: make([]FoundServer, len(servers)), People: make([]FoundPerson, len(people))}
	for index, server := range servers {
		found.Servers[index] = FoundServer(server)
	}
	for index, person := range people {
		found.People[index] = FoundPerson(person)
	}
	return found, nil
}

// visibleChannels returns the matching channels the user may see, at most 20.
func (service *Service) visibleChannels(ctx context.Context, userID uuid.UUID, like string) ([]FoundChannel, error) {
	rows, err := service.store.FindChannels(ctx, store.FindChannelsParams{UserID: userID, Pattern: like})
	if err != nil {
		return nil, fmt.Errorf("finding channels: %w", err)
	}
	accesses := map[uuid.UUID]access{}
	visible := []FoundChannel{}
	for _, row := range rows {
		a, known := accesses[row.ServerID]
		if !known {
			if a, err = service.load(ctx, Scope{ServerID: row.ServerID, UserID: userID}); err != nil {
				return nil, err
			}
			accesses[row.ServerID] = a
		}
		if a.inChannel(row.ID).Has(permissions.ViewChannel) && len(visible) < 20 {
			visible = append(visible, FoundChannel(row))
		}
	}
	return visible, nil
}
