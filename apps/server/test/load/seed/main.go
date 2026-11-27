// Command seed prepares an instance's database for the load smoke test
// (tests/load, spec §7.2): users with sessions, all members of one server,
// and an application message the k6 script sends in its #general channel.
// It prints what the script needs as JSON.
//
// It writes to the database directly, as the instance itself would, so
// that hundreds of accounts do not have to pass sign-up and its rate
// limits. Run it against a test instance only.
//
// Usage: DATABASE_URL=… seed -users 500 > seed.json
package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	accountsstore "github.com/aazam476/nexus-chat/apps/server/internal/accounts/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	sessionstore "github.com/aazam476/nexus-chat/apps/server/internal/auth/session/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/database"
	"github.com/aazam476/nexus-chat/apps/server/internal/servers"
	serversstore "github.com/aazam476/nexus-chat/apps/server/internal/servers/store"
)

// Seed is what the k6 script reads.
type Seed struct {
	// Tokens are the users' session tokens; the first user owns the server and sends.
	Tokens    []string `json:"tokens"`
	ServerID  string   `json:"serverId"`
	ChannelID string   `json:"channelId"`
	// AudienceID is the conversation whose group carries the channel.
	AudienceID string `json:"audienceId"`
	// Message is an MLS application message for the group's first epoch, base64.
	Message string `json:"message"`
}

func main() {
	users := flag.Int("users", 500, "how many users to create")
	flag.Parse()
	if err := run(context.Background(), os.Getenv("DATABASE_URL"), *users); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, databaseURL string, users int) error {
	if databaseURL == "" || users < 1 {
		return errors.New("set DATABASE_URL and at least one user")
	}
	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	seed, err := build(ctx, pool, users)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(os.Stdout).Encode(seed); err != nil {
		return fmt.Errorf("writing the seed: %w", err)
	}
	return nil
}

// build makes the users, their sessions and the server.
func build(ctx context.Context, pool *pgxpool.Pool, users int) (Seed, error) {
	accounts, sessions := accountsstore.New(pool), session.NewService(sessionstore.New(pool), clock.System{})
	ids := make([]uuid.UUID, users)
	seed := Seed{Tokens: make([]string, users)}
	run := uuid.New().String()[:8]
	for index := range users {
		ids[index] = uuid.New()
		name := fmt.Sprintf("load_%s_%d", run, index)
		err := accounts.InsertUser(ctx, accountsstore.InsertUserParams{ID: ids[index], Username: name, DisplayName: name, Email: name + "@load.invalid", CreatedAt: time.Now()})
		if err != nil {
			return Seed{}, fmt.Errorf("creating user %d: %w", index, err)
		}
		if seed.Tokens[index], _, err = sessions.Create(ctx, ids[index], session.Device{Name: "load test"}); err != nil {
			return Seed{}, err
		}
	}
	return seed, joinServer(ctx, pool, ids, &seed)
}

// joinServer makes a server owned by the first user, and has every other user join it.
func joinServer(ctx context.Context, pool *pgxpool.Pool, ids []uuid.UUID, seed *Seed) error {
	service := servers.NewService(servers.Dependencies{
		Store: serversstore.NewWriter(pool), Notifier: quiet{}, Audiences: quiet{},
		Icons: servers.Icons{Logger: slog.New(slog.DiscardHandler)}, Clock: clock.System{},
	})
	details, err := service.Create(ctx, ids[0], "Load test", "")
	if err != nil {
		return err
	}
	invite, err := service.CreateInvite(ctx, servers.Scope{ServerID: details.Server.ID, UserID: ids[0]}, servers.InviteLimits{})
	if err != nil {
		return err
	}
	for _, member := range ids[1:] {
		if _, err := service.Join(ctx, invite.Code, member); err != nil {
			return fmt.Errorf("joining: %w", err)
		}
	}
	current, err := service.Details(ctx, servers.Scope{ServerID: details.Server.ID, UserID: ids[0]})
	if err != nil {
		return err
	}
	for _, channel := range current.Channels {
		if channel.Kind == servers.KindText && channel.AudienceID != nil {
			seed.ServerID, seed.ChannelID, seed.AudienceID = details.Server.ID.String(), channel.ID.String(), channel.AudienceID.String()
			seed.Message = base64.StdEncoding.EncodeToString(applicationMessage(channel.MLSGroupID))
			return nil
		}
	}
	return errors.New("the new server has no text channel")
}

// applicationMessage is an MLS 1.0 PrivateMessage for the group's first
// epoch with application content (RFC 9420 §6.3): what the delivery
// service checks. Its encrypted parts are placeholders; nobody decrypts it.
func applicationMessage(groupID []byte) []byte {
	// Group IDs are 16 bytes, so their length fits the one-byte vector prefix.
	message := []byte{0, 1, 0, 2, byte(len(groupID) & 0x3f)}
	message = append(message, groupID...)
	message = binary.BigEndian.AppendUint64(message, 0)
	return append(message, 1, 0, 1, 0xde, 1, 0xad)
}

// quiet tells nobody about changes: the instance under test reads them
// from the database.
type quiet struct{}

func (quiet) ServerChanged(context.Context, []uuid.UUID, uuid.UUID) {}

func (quiet) MemberLeft(context.Context, uuid.UUID, uuid.UUID) error { return nil }
