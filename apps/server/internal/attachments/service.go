package attachments

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aazam476/nexus-chat/apps/server/internal/attachments/store"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/filestore"
)

// Errors returned to the caller.
var (
	ErrNotFound    = errors.New("there is no such attachment, or you are not in its conversation")
	ErrNotAllowed  = errors.New("you cannot send in this conversation")
	ErrTooLarge    = errors.New("the file is larger than this instance allows")
	ErrInvalidSize = errors.New("the declared size must be positive and match the file")
	ErrRateLimited = errors.New("too many uploads; wait a minute")
)

// Attachment is a stored encrypted file.
type Attachment struct {
	ID   uuid.UUID
	Size int64
}

// Membership is what the service asks the conversations feature.
type Membership interface {
	IsMember(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) (bool, error)
	MaySend(ctx context.Context, conversationID uuid.UUID, userID uuid.UUID) (bool, error)
}

// Limits gives the instance's attachment limit (plaintext bytes).
type Limits interface {
	AttachmentMaxBytes(ctx context.Context) (int64, error)
}

type attachmentStore interface {
	InsertAttachment(ctx context.Context, params store.InsertAttachmentParams) error
	AttachmentSize(ctx context.Context, params store.AttachmentSizeParams) (int64, error)
}

type fileStore interface {
	SaveStream(ctx context.Context, key string, content io.Reader, limit int64) (int64, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type attemptLimiter interface {
	Allow(key string) bool
}

// Dependencies are what the service needs.
type Dependencies struct {
	Store   attachmentStore
	Files   fileStore
	Members Membership
	Limits  Limits
	Limiter attemptLimiter
	Clock   clock.Clock
}

// Service stores and hands out encrypted attachments.
type Service struct {
	dependencies Dependencies
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies}
}

func fileKey(id uuid.UUID) string {
	return "attachments/" + id.String()
}

// CiphertextAllowance is the largest encrypted file for a plaintext limit:
// the secretstream header and, per 1 MiB piece, its 17-byte overhead; this
// also covers one-shot AES-GCM's 16-byte tag (docs/e2ee-design.md §7).
func CiphertextAllowance(limit int64) int64 {
	const piece = 1 << 20
	pieces := (limit + piece - 1) / piece
	return limit + 24 + 17*(pieces+1)
}

// Upload is an encrypted file a member sends to a conversation. Size is
// the size the client declared; the stored file must be exactly that long.
type Upload struct {
	ConversationID uuid.UUID
	Size           int64
	Content        io.Reader
}

// Upload stores a member's encrypted file in its conversation.
func (service *Service) Upload(ctx context.Context, me uuid.UUID, upload Upload) (Attachment, error) {
	conversationID, size := upload.ConversationID, upload.Size
	if err := service.checkUpload(ctx, me, conversationID, size); err != nil {
		return Attachment{}, err
	}
	id := uuid.New()
	written, err := service.dependencies.Files.SaveStream(ctx, fileKey(id), upload.Content, size)
	if errors.Is(err, filestore.ErrTooLarge) {
		return Attachment{}, ErrInvalidSize
	}
	if err != nil {
		return Attachment{}, err
	}
	if written != size {
		_ = service.dependencies.Files.Delete(ctx, fileKey(id))
		return Attachment{}, ErrInvalidSize
	}
	err = service.dependencies.Store.InsertAttachment(ctx, store.InsertAttachmentParams{
		ID: id, ConversationID: conversationID, UploaderID: &me, Size: size, CreatedAt: service.dependencies.Clock.Now(),
	})
	if err != nil {
		_ = service.dependencies.Files.Delete(ctx, fileKey(id))
		return Attachment{}, fmt.Errorf("recording attachment: %w", err)
	}
	return Attachment{ID: id, Size: size}, nil
}

// checkUpload refuses an upload before any of it is read.
func (service *Service) checkUpload(ctx context.Context, me uuid.UUID, conversationID uuid.UUID, size int64) error {
	if size <= 0 {
		return ErrInvalidSize
	}
	allowed, err := service.dependencies.Members.MaySend(ctx, conversationID, me)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrNotAllowed
	}
	limit, err := service.dependencies.Limits.AttachmentMaxBytes(ctx)
	if err != nil {
		return err
	}
	if size > CiphertextAllowance(limit) {
		return ErrTooLarge
	}
	if !service.dependencies.Limiter.Allow(me.String()) {
		return ErrRateLimited
	}
	return nil
}

// Download is a stored attachment ready to send.
type Download struct {
	Content io.ReadCloser
	Size    int64
}

// Open returns an attachment of a conversation the caller is in.
func (service *Service) Open(ctx context.Context, me uuid.UUID, conversationID uuid.UUID, id uuid.UUID) (Download, error) {
	isMember, err := service.dependencies.Members.IsMember(ctx, conversationID, me)
	if err != nil {
		return Download{}, err
	}
	if !isMember {
		return Download{}, ErrNotFound
	}
	size, err := service.dependencies.Store.AttachmentSize(ctx, store.AttachmentSizeParams{ID: id, ConversationID: conversationID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Download{}, ErrNotFound
	}
	if err != nil {
		return Download{}, err
	}
	content, err := service.dependencies.Files.Open(ctx, fileKey(id))
	if err != nil {
		return Download{}, fmt.Errorf("opening attachment: %w", err)
	}
	return Download{Content: content, Size: size}, nil
}
