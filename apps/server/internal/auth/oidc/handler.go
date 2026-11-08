package oidc

import (
	"context"
	"errors"

	"github.com/aazam476/nexus-chat/apps/server/internal/accounts"
	"github.com/aazam476/nexus-chat/apps/server/internal/api"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/login"
	"github.com/aazam476/nexus-chat/apps/server/internal/auth/session"
	"github.com/aazam476/nexus-chat/apps/server/internal/instanceconfig"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/apiresponse"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/clientip"
	"github.com/aazam476/nexus-chat/apps/server/internal/platform/validation"
)

// errNoPrincipal means a protected handler ran without authentication,
// which the session middleware makes impossible.
var errNoPrincipal = errors.New("handler requires an authenticated request")

type baseURLSource interface {
	BaseURL(ctx context.Context) (string, error)
}

// Handler serves the provider sign-in endpoints and their admin settings.
type Handler struct {
	signIn    *SignIn
	providers *Providers
	users     accounts.UserReader
	settings  baseURLSource
}

// NewHandler returns a Handler. users checks admin rights; settings gives
// the base URL for the callback address shown to admins.
func NewHandler(signIn *SignIn, providers *Providers, users accounts.UserReader, settings baseURLSource) *Handler {
	return &Handler{signIn: signIn, providers: providers, users: users, settings: settings}
}

// ListSignInProviders answers GET /api/v1/auth/oidc/providers.
func (handler *Handler) ListSignInProviders(ctx context.Context, _ api.ListSignInProvidersRequestObject) (api.ListSignInProvidersResponseObject, error) {
	providers, err := handler.providers.List(ctx)
	if err != nil {
		return nil, err
	}
	response := make(api.ListSignInProviders200JSONResponse, 0, len(providers))
	for _, provider := range providers {
		response = append(response, api.SignInProvider{Id: provider.ID, DisplayName: provider.DisplayName})
	}
	return response, nil
}

// beginFailure is the API error for a failed Begin, or ok false when the
// error is unexpected.
type beginFailure struct {
	status int
	body   api.Error
}

func classifyBegin(err error) (beginFailure, bool) {
	var problems *validation.Error
	var unavailable *UnavailableError
	switch {
	case errors.Is(err, ErrProviderNotFound):
		return beginFailure{404, apiresponse.Error("provider_not_found", err)}, true
	case errors.Is(err, instanceconfig.ErrNoDomain):
		return beginFailure{409, apiresponse.Error("domain_required", err)}, true
	case errors.As(err, &problems):
		return beginFailure{422, apiresponse.Validation(problems)}, true
	case errors.Is(err, ErrRateLimited):
		return beginFailure{429, apiresponse.Error("rate_limited", err)}, true
	case errors.As(err, &unavailable):
		return beginFailure{502, apiresponse.Error("provider_unavailable", unavailable)}, true
	default:
		return beginFailure{}, false
	}
}

// BeginProviderSignIn answers POST /api/v1/auth/oidc/providers/{providerId}/begin.
func (handler *Handler) BeginProviderSignIn(ctx context.Context, request api.BeginProviderSignInRequestObject) (api.BeginProviderSignInResponseObject, error) {
	beginRequest := BeginRequest{ProviderID: request.ProviderId, ReturnTo: request.Body.ReturnTo, ClientKey: clientKey(ctx)}
	if request.Body.InviteCode != nil {
		beginRequest.InviteCode = *request.Body.InviteCode
	}
	started, err := handler.signIn.Begin(ctx, beginRequest)
	if err == nil {
		return api.BeginProviderSignIn200JSONResponse(providerFlow(started)), nil
	}
	failure, known := classifyBegin(err)
	if !known {
		return nil, err
	}
	switch failure.status {
	case 404:
		return api.BeginProviderSignIn404JSONResponse{ProviderNotFoundJSONResponse: api.ProviderNotFoundJSONResponse(failure.body)}, nil
	case 409:
		return api.BeginProviderSignIn409JSONResponse{DomainRequiredJSONResponse: api.DomainRequiredJSONResponse(failure.body)}, nil
	case 422:
		return api.BeginProviderSignIn422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(failure.body)}, nil
	case 429:
		return api.BeginProviderSignIn429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(failure.body)}, nil
	default:
		return api.BeginProviderSignIn502JSONResponse{ProviderUnavailableJSONResponse: api.ProviderUnavailableJSONResponse(failure.body)}, nil
	}
}

// BeginProviderLink answers POST /api/v1/auth/oidc/providers/{providerId}/link.
func (handler *Handler) BeginProviderLink(ctx context.Context, request api.BeginProviderLinkRequestObject) (api.BeginProviderLinkResponseObject, error) {
	principal, ok := session.PrincipalFrom(ctx)
	if !ok {
		return nil, errNoPrincipal
	}
	// A linked provider is a way to log in, which a stolen session must not add.
	if !handler.recent(principal) {
		return api.BeginProviderLink403JSONResponse{ReauthenticationRequiredJSONResponse: reauthenticationRequired()}, nil
	}
	started, err := handler.signIn.Begin(ctx, BeginRequest{
		ProviderID: request.ProviderId, ReturnTo: request.Body.ReturnTo, LinkUserID: &principal.UserID, ClientKey: clientKey(ctx),
	})
	if err == nil {
		return api.BeginProviderLink200JSONResponse(providerFlow(started)), nil
	}
	failure, known := classifyBegin(err)
	if !known {
		return nil, err
	}
	switch failure.status {
	case 404:
		return api.BeginProviderLink404JSONResponse{ProviderNotFoundJSONResponse: api.ProviderNotFoundJSONResponse(failure.body)}, nil
	case 409:
		return api.BeginProviderLink409JSONResponse{DomainRequiredJSONResponse: api.DomainRequiredJSONResponse(failure.body)}, nil
	case 422:
		return api.BeginProviderLink422JSONResponse{ValidationFailedJSONResponse: api.ValidationFailedJSONResponse(failure.body)}, nil
	case 429:
		return api.BeginProviderLink429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(failure.body)}, nil
	default:
		return api.BeginProviderLink502JSONResponse{ProviderUnavailableJSONResponse: api.ProviderUnavailableJSONResponse(failure.body)}, nil
	}
}

func providerFlow(started Started) api.ProviderFlow {
	return api.ProviderFlow{FlowId: started.FlowID, AuthorizationUrl: started.AuthorizationURL}
}

// ProviderCallback answers GET /api/v1/auth/oidc/callback.
func (handler *Handler) ProviderCallback(ctx context.Context, request api.ProviderCallbackRequestObject) (api.ProviderCallbackResponseObject, error) {
	location, err := handler.signIn.Callback(ctx, CallbackRequest{
		State: value(request.Params.State), Code: value(request.Params.Code), Error: value(request.Params.Error),
	})
	if errors.Is(err, ErrInvalidState) {
		return api.ProviderCallback400JSONResponse(apiresponse.Error("invalid_state", err)), nil
	}
	if err != nil {
		return nil, err
	}
	return api.ProviderCallback302Response{Headers: api.ProviderCallback302ResponseHeaders{Location: location}}, nil
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}

func clientKey(ctx context.Context) string {
	return clientip.FromContext(ctx).Address.String()
}

// completeCodes maps sign-in refusals to their API codes.
var completeCodes = map[error]string{ //nolint:gochecknoglobals // constant lookup table
	ErrNoAccount: "no_account", ErrRegistrationClosed: "registration_closed", ErrInviteRequired: "invite_required",
	ErrInvalidInvite: "invalid_invite", ErrEmailRequired: "email_required",
}

// CompleteProviderSignIn answers POST /api/v1/auth/oidc/complete.
func (handler *Handler) CompleteProviderSignIn(ctx context.Context, request api.CompleteProviderSignInRequestObject) (api.CompleteProviderSignInResponseObject, error) {
	result, err := handler.signIn.Complete(ctx, CompleteRequest{
		FlowID: request.Body.FlowId, Code: request.Body.Code,
		Device: session.Device{Name: apiresponse.DeviceName(request.Body.DeviceName)}, ClientKey: clientKey(ctx),
	})
	for refusal, code := range completeCodes {
		if errors.Is(err, refusal) {
			return api.CompleteProviderSignIn403JSONResponse(apiresponse.Error(code, err)), nil
		}
	}
	switch {
	case errors.Is(err, ErrInvalidFlow):
		return api.CompleteProviderSignIn401JSONResponse(apiresponse.Error("invalid_flow", err)), nil
	case errors.Is(err, ErrEmailInUse):
		return api.CompleteProviderSignIn409JSONResponse(apiresponse.Error("email_in_use", err)), nil
	case errors.Is(err, ErrIdentityInUse):
		return api.CompleteProviderSignIn409JSONResponse(apiresponse.Error("identity_in_use", err)), nil
	case errors.Is(err, ErrRateLimited):
		return api.CompleteProviderSignIn429JSONResponse{RateLimitedJSONResponse: api.RateLimitedJSONResponse(apiresponse.Error("rate_limited", err))}, nil
	case err != nil:
		return nil, err
	case result.MFAChallenge != "":
		return api.CompleteProviderSignIn202JSONResponse(mfaChallenge(result)), nil
	}
	return api.CompleteProviderSignIn200JSONResponse{
		Token: result.Token, Session: apiresponse.Session(result.Session, result.Session.ID), User: accounts.CurrentUserResponse(result.User),
	}, nil
}

func mfaChallenge(result login.Result) api.MfaChallenge {
	methods := make([]api.MfaMethod, 0, len(result.MFAMethods))
	for _, method := range result.MFAMethods {
		methods = append(methods, api.MfaMethod(method))
	}
	return api.MfaChallenge{ChallengeToken: result.MFAChallenge, Methods: methods}
}

// recent reports whether principal logged in recently enough to change how
// the account logs in (session.Principal.RequireRecent).
func (handler *Handler) recent(principal session.Principal) bool {
	return principal.RequireRecent(handler.signIn.dependencies.Clock.Now()) == nil
}

// reauthenticationRequired is the body of a 403 that asks for a recent login.
func reauthenticationRequired() api.ReauthenticationRequiredJSONResponse {
	return api.ReauthenticationRequiredJSONResponse(apiresponse.Error("reauthentication_required", session.ErrReauthenticationRequired))
}
