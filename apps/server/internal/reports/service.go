package reports

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/delivery"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clock"
	"github.com/aazam476/nexus-chat/apps/server/internal/reports/store"
)

// Limits on what a report holds.
const (
	maximumComment = 1000
	uniqueViolate  = "23505"
)

// Errors returned to the caller.
var (
	ErrNotFound  = errors.New("there is no such message you can read, or no such report")
	ErrForbidden = errors.New("you cannot handle this server's reports")
	ErrOwn       = errors.New("you cannot report your own message")
	ErrReported  = errors.New("you already reported this message")
	ErrInvalid   = errors.New("a comment or resolution is at most 1000 characters")
	ErrHandled   = errors.New("this report was already handled")
	ErrNoServer  = errors.New("there is no such server, or you are not in it")
)

// Statuses of a report.
const (
	StatusOpen     = "open"
	StatusResolved = "resolved"
)

// Copies finds the stored archive copy of a message the reporter can read;
// *delivery.Service implements it.
type Copies interface {
	ChannelCopy(ctx context.Context, ref delivery.ChannelRef, messageID uuid.UUID) (delivery.StoredCopy, error)
	ConversationCopy(ctx context.Context, me uuid.UUID, conversationID uuid.UUID, messageID uuid.UUID) (delivery.StoredCopy, error)
}

// Moderation answers who handles a server's reports and records what they
// do in its audit log; *servers.Service implements it.
type Moderation interface {
	// MayHandleReports is MANAGE_MESSAGES in the server; it fails with
	// ErrNoServer for someone not in it.
	MayHandleReports(ctx context.Context, serverID uuid.UUID, userID uuid.UUID) (bool, error)
	ReportHandled(ctx context.Context, handled Handled) error
}

// Handled is what a moderator did with a report, for the audit log.
type Handled struct {
	ServerID, ActorID, ReportID uuid.UUID
	// Escalated is true when they passed it on, false when they resolved it.
	Escalated bool
}

// Settings says whether every report also goes to the instance admins,
// and the instance's domain, which senders' addresses end in.
type Settings interface {
	ReportsToAdmins(ctx context.Context) (bool, error)
	Domain(ctx context.Context) (string, error)
}

// Dependencies are what the service needs. Key is the instance's key, the
// one that signs membership directives.
type Dependencies struct {
	Store      *store.Queries
	Copies     Copies
	Moderation Moderation
	Settings   Settings
	Users      accounts.UserReader
	Key        ed25519.PrivateKey
	Clock      clock.Clock
}

// Service handles reports.
type Service struct {
	dependencies Dependencies
}

// NewService returns a Service.
func NewService(dependencies Dependencies) *Service {
	return &Service{dependencies: dependencies}
}

// Target is the reported message: in a server channel (ServerID and
// ChannelID) or a direct or group conversation (ConversationID).
type Target struct {
	ServerID       *uuid.UUID
	ChannelID      *uuid.UUID
	ConversationID *uuid.UUID
	MessageID      uuid.UUID
}

// NewReport is what a reporter sends.
type NewReport struct {
	Target   Target
	Evidence []byte
	Comment  string
}

// Submit files a report about a message the reporter can read, with the
// instance's receipt for its stored copy, and returns the report's ID.
func (service *Service) Submit(ctx context.Context, reporterID uuid.UUID, report NewReport) (uuid.UUID, error) {
	if len([]rune(report.Comment)) > maximumComment {
		return uuid.Nil, ErrInvalid
	}
	found, place, err := service.copyFor(ctx, reporterID, report.Target)
	if err != nil {
		return uuid.Nil, err
	}
	if found.SenderID == nil {
		return uuid.Nil, ErrNotFound
	}
	if *found.SenderID == reporterID {
		return uuid.Nil, ErrOwn
	}
	if err := checkEvidence(report.Evidence, evidenceAbout{place: place, message: found.MessageID, epoch: found.Epoch}); err != nil {
		return uuid.Nil, err
	}
	sender, err := service.address(ctx, *found.SenderID)
	if err != nil {
		return uuid.Nil, err
	}
	signed, err := SignReceipt(service.dependencies.Key, Receipt{
		ConversationID: found.ConversationID, ChannelID: found.ChannelID, MessageID: found.MessageID, SenderID: *found.SenderID, Sender: sender,
		Epoch: found.Epoch, ArchiveHash: archiveHash(found.Sealed), SentAt: found.CreatedAt, IssuedAt: service.dependencies.Clock.Now(),
	})
	if err != nil {
		return uuid.Nil, err
	}
	return service.insert(ctx, reporterID, report, filed{copy: found, receipt: signed})
}

// copyFor finds the reported message's copy, and the ID its archive copy
// is addressed by: the channel's for a channel message, else the conversation's.
func (service *Service) copyFor(ctx context.Context, reporterID uuid.UUID, target Target) (delivery.StoredCopy, uuid.UUID, error) {
	var found delivery.StoredCopy
	var place uuid.UUID
	var err error
	switch {
	case target.ServerID != nil && target.ChannelID != nil && target.ConversationID == nil:
		place = *target.ChannelID
		found, err = service.dependencies.Copies.ChannelCopy(ctx, delivery.ChannelRef{ServerID: *target.ServerID, ChannelID: place, UserID: reporterID}, target.MessageID)
	case target.ServerID == nil && target.ChannelID == nil && target.ConversationID != nil:
		place = *target.ConversationID
		found, err = service.dependencies.Copies.ConversationCopy(ctx, reporterID, place, target.MessageID)
	default:
		return delivery.StoredCopy{}, uuid.Nil, ErrNotFound
	}
	if errors.Is(err, delivery.ErrNotFound) || errors.Is(err, delivery.ErrNoMessage) {
		return delivery.StoredCopy{}, uuid.Nil, ErrNotFound
	}
	return found, place, err
}

// filed is the stored copy a report is about and the receipt for it.
type filed struct {
	copy    delivery.StoredCopy
	receipt SignedReceipt
}

func (service *Service) insert(ctx context.Context, reporterID uuid.UUID, report NewReport, about filed) (uuid.UUID, error) {
	id := uuid.Must(uuid.NewV7())
	params := store.InsertReportParams{
		ID: id, ServerID: report.Target.ServerID, ChannelID: about.copy.ChannelID, ConversationID: &about.copy.ConversationID,
		MessageID: about.copy.MessageID, ReporterID: &reporterID, SenderID: about.copy.SenderID, Evidence: report.Evidence,
		Receipt: about.receipt.Receipt, ReceiptSig: about.receipt.Signature, Comment: report.Comment, CreatedAt: service.dependencies.Clock.Now(),
	}
	err := service.dependencies.Store.InsertReport(ctx, params)
	if pgErr := (*pgconn.PgError)(nil); errors.As(err, &pgErr) && pgErr.Code == uniqueViolate {
		return uuid.Nil, ErrReported
	}
	return id, err
}

// Report is a filed report as moderators see it.
type Report = store.Report

// handling requires that userID may handle the server's reports.
func (service *Service) handling(ctx context.Context, serverID uuid.UUID, userID uuid.UUID) error {
	may, err := service.dependencies.Moderation.MayHandleReports(ctx, serverID, userID)
	if err != nil {
		return err
	}
	if !may {
		return ErrForbidden
	}
	return nil
}

// ServerReports lists a server's reports with a status, newest first, to
// its moderators (MANAGE_MESSAGES).
func (service *Service) ServerReports(ctx context.Context, serverID uuid.UUID, userID uuid.UUID, status string) ([]Report, error) {
	if err := service.handling(ctx, serverID, userID); err != nil {
		return nil, err
	}
	return service.dependencies.Store.ServerReports(ctx, store.ServerReportsParams{ServerID: &serverID, Status: status})
}

// Action is a moderator resolving or passing on a report.
type Action struct {
	ServerID, UserID, ReportID uuid.UUID
	// Resolution is what was done, when resolving.
	Resolution string
}

// serverReport returns one of the server's reports to a moderator.
func (service *Service) serverReport(ctx context.Context, action Action) (Report, error) {
	if err := service.handling(ctx, action.ServerID, action.UserID); err != nil {
		return Report{}, err
	}
	report, err := service.report(ctx, action.ReportID)
	if err != nil {
		return Report{}, err
	}
	if report.ServerID == nil || *report.ServerID != action.ServerID {
		return Report{}, ErrNotFound
	}
	return report, nil
}

// ResolveServerReport closes one of the server's open reports, noting what was done.
func (service *Service) ResolveServerReport(ctx context.Context, action Action) error {
	if _, err := service.serverReport(ctx, action); err != nil {
		return err
	}
	if err := service.resolve(ctx, action.ReportID, action.UserID, action.Resolution); err != nil {
		return err
	}
	return service.dependencies.Moderation.ReportHandled(ctx, Handled{ServerID: action.ServerID, ActorID: action.UserID, ReportID: action.ReportID})
}

// EscalateServerReport passes an open report on to the instance admins.
func (service *Service) EscalateServerReport(ctx context.Context, action Action) error {
	if _, err := service.serverReport(ctx, action); err != nil {
		return err
	}
	changed, err := service.dependencies.Store.EscalateReport(ctx, action.ReportID)
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrHandled
	}
	return service.dependencies.Moderation.ReportHandled(ctx, Handled{ServerID: action.ServerID, ActorID: action.UserID, ReportID: action.ReportID, Escalated: true})
}

func (service *Service) resolve(ctx context.Context, reportID uuid.UUID, userID uuid.UUID, resolution string) error {
	if len([]rune(resolution)) > maximumComment {
		return ErrInvalid
	}
	now := service.dependencies.Clock.Now()
	changed, err := service.dependencies.Store.ResolveReport(ctx, store.ResolveReportParams{
		ID: reportID, Resolution: resolution, ResolvedBy: &userID, ResolvedAt: &now,
	})
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrHandled
	}
	return nil
}

// AdminReports lists the reports for the instance admins with a status:
// those about direct and group messages, those moderators passed on, and
// every other one too when the instance asks for it. Callers check that
// the user is an instance admin.
func (service *Service) AdminReports(ctx context.Context, status string) ([]Report, error) {
	everything, err := service.dependencies.Settings.ReportsToAdmins(ctx)
	if err != nil {
		return nil, err
	}
	return service.dependencies.Store.AdminReports(ctx, store.AdminReportsParams{Status: status, Everything: everything})
}

// ResolveAdminReport closes a report as an instance admin. Callers check
// that the user is one.
func (service *Service) ResolveAdminReport(ctx context.Context, adminID uuid.UUID, reportID uuid.UUID, resolution string) error {
	report, err := service.report(ctx, reportID)
	if err != nil {
		return err
	}
	everything, err := service.dependencies.Settings.ReportsToAdmins(ctx)
	if err != nil {
		return err
	}
	if report.ServerID != nil && !report.Escalated && !everything {
		return ErrNotFound
	}
	return service.resolve(ctx, reportID, adminID, resolution)
}

// report reads one report, or ErrNotFound.
func (service *Service) report(ctx context.Context, reportID uuid.UUID) (Report, error) {
	report, err := service.dependencies.Store.GetReport(ctx, reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Report{}, ErrNotFound
	}
	return report, err
}

// address is a user's @name@domain on this instance.
func (service *Service) address(ctx context.Context, userID uuid.UUID) (string, error) {
	user, err := service.dependencies.Users.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("loading the sender: %w", err)
	}
	domain, err := service.dependencies.Settings.Domain(ctx)
	if err != nil {
		return "", err
	}
	return "@" + user.Username + "@" + domain, nil
}
