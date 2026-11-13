package servers

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/permissions"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
)

// errNoPrincipal means a protected handler ran without authentication,
// which the session middleware makes impossible; it is a programming error.
var errNoPrincipal = errors.New("handler requires an authenticated request")

// Handler serves the server, channel, role and invite endpoints.
type Handler struct {
	service *Service
}

// NewHandler returns a Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func caller(ctx context.Context) (uuid.UUID, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return uuid.Nil, errNoPrincipal
	}
	return principal.UserID, nil
}

// scope is the caller acting in serverID.
func scope(ctx context.Context, serverID uuid.UUID) (Scope, error) {
	me, err := caller(ctx)
	return Scope{ServerID: serverID, UserID: me}, err
}

// errorBody is any generated error response: each operation has its own
// named copy of api.Error.
type errorBody interface {
	~struct {
		Code    string            `json:"code"`
		Fields  *[]api.FieldError `json:"fields,omitempty"`
		Message string            `json:"message"`
	}
}

// refusalCode is a service error the contract answers, with its status and code.
type refusalCode struct {
	err    error
	status int
	code   string
}

// refusals are the service errors the contract answers.
func refusals() []refusalCode {
	return []refusalCode{
		{ErrForbidden, http.StatusForbidden, "missing_permission"},
		{ErrNotFound, http.StatusNotFound, "server_not_found"},
		{ErrNoChannel, http.StatusNotFound, "channel_not_found"},
		{ErrNoRole, http.StatusNotFound, "role_not_found"},
		{ErrNoMember, http.StatusNotFound, "member_not_found"},
		{ErrInviteInvalid, http.StatusNotFound, "invite_invalid"},
		{ErrInvalid, http.StatusUnprocessableEntity, "invalid"},
		{ErrBadOrder, http.StatusUnprocessableEntity, "bad_order"},
		{ErrBadPermission, http.StatusUnprocessableEntity, "unknown_permission"},
		{ErrOwnerLeaving, http.StatusUnprocessableEntity, "owner_cannot_leave"},
	}
}

// refusal turns a service error the caller can act on into the
// operation's 403 (F), 404 (N) or 422 (I) response, as R. Other errors,
// and nil, are not refusals.
func refusal[R any, F, N, I errorBody](err error) (R, bool) {
	var zero R
	for _, known := range refusals() {
		if !errors.Is(err, known.err) {
			continue
		}
		body := apiresponse.Error(known.code, err)
		var out any
		switch known.status {
		case http.StatusForbidden:
			out = F(body)
		case http.StatusNotFound:
			out = N(body)
		default:
			out = I(body)
		}
		response, ok := out.(R)
		return response, ok
	}
	return zero, false
}

// permissionNames lists p's names for the API.
func permissionNames(p permissions.Permission) []api.PermissionName {
	names := p.Strings()
	out := make([]api.PermissionName, 0, len(names))
	for _, name := range names {
		out = append(out, api.PermissionName(name))
	}
	return out
}

// parsePermissions reads names from the API; unknown names are refused.
func parsePermissions(names []api.PermissionName) (permissions.Permission, error) {
	plain := make([]string, 0, len(names))
	for _, name := range names {
		plain = append(plain, string(name))
	}
	p, ok := permissions.Parse(plain)
	if !ok {
		return 0, ErrBadPermission
	}
	return p, nil
}
