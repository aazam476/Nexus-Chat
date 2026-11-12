package linkpreview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// Limits on fetching.
const (
	maximumPageBytes  = 1 << 20
	maximumImageBytes = 5 << 20
	maximumRedirects  = 3
	fetchTimeout      = 8 * time.Second
)

// Errors returned to the caller.
var (
	ErrDisabled    = errors.New("the instance's link preview proxy is off")
	ErrInvalidLink = errors.New("previews are only made for public http and https links")
	ErrUnavailable = errors.New("the page could not be fetched or has nothing to preview")
	ErrRateLimited = errors.New("too many previews; wait a minute")
)

// Switch says whether the admin turned the proxy on.
type Switch interface {
	LinkPreviewProxy(ctx context.Context) (bool, error)
}

type attemptLimiter interface {
	Allow(key string) bool
}

// Image is a preview image as fetched.
type Image struct {
	Data        []byte
	ContentType string
}

// Service fetches previews for users who ask, while the proxy is on.
type Service struct {
	client  *http.Client
	toggle  Switch
	limiter attemptLimiter
}

// Options configure NewService. Allow decides which addresses may be
// reached; nil means public ones only (tests allow their own server).
type Options struct {
	Switch  Switch
	Limiter attemptLimiter
	Allow   func(netip.Addr) bool
}

// NewService returns a Service whose fetches reach only allowed addresses.
func NewService(options Options) *Service {
	allow := options.Allow
	if allow == nil {
		allow = publicAddress
	}
	transport := &http.Transport{DialContext: guardedDialer(allow).DialContext, Proxy: nil, ResponseHeaderTimeout: fetchTimeout}
	client := &http.Client{
		Transport: transport,
		Timeout:   fetchTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maximumRedirects || !allowedLink(request.URL) {
				return ErrUnavailable
			}
			return nil
		},
	}
	return &Service{client: client, toggle: options.Switch, limiter: options.Limiter}
}

func allowedLink(link *url.URL) bool {
	return (link.Scheme == "https" || link.Scheme == "http") && link.User == nil && link.Hostname() != ""
}

// check refuses a fetch while the proxy is off, for a bad link, or too often.
func (service *Service) check(ctx context.Context, userKey string, raw string) (*url.URL, error) {
	enabled, err := service.toggle.LinkPreviewProxy(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrDisabled
	}
	link, err := url.Parse(raw)
	if err != nil || !allowedLink(link) {
		return nil, ErrInvalidLink
	}
	if !service.limiter.Allow(userKey) {
		return nil, ErrRateLimited
	}
	return link, nil
}

// get fetches link and returns at most limit+1 bytes of its body, so a
// caller can tell a body over the limit, when its content type passes
// accept.
func (service *Service) get(ctx context.Context, link *url.URL, limit int64, accept func(string) bool) ([]byte, string, *url.URL, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, link.String(), nil)
	if err != nil {
		return nil, "", nil, ErrInvalidLink
	}
	request.Header.Set("User-Agent", "NexusChatLinkPreview/1")
	response, err := service.client.Do(request)
	if err != nil {
		return nil, "", nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer func() { _ = response.Body.Close() }()
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if response.StatusCode != http.StatusOK || !accept(mediaType) {
		return nil, "", nil, ErrUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, "", nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return body, mediaType, response.Request.URL, nil
}

// Preview returns what a page shows in a preview.
func (service *Service) Preview(ctx context.Context, userKey string, raw string) (Page, error) {
	link, err := service.check(ctx, userKey, raw)
	if err != nil {
		return Page{}, err
	}
	body, _, final, err := service.get(ctx, link, maximumPageBytes, func(mediaType string) bool {
		return mediaType == "text/html" || mediaType == "application/xhtml+xml"
	})
	if err != nil {
		return Page{}, err
	}
	// The head comes first, so a page cut off at the limit still previews.
	page := parsePage(strings.NewReader(string(body)), final)
	if page.Title == "" && page.Description == "" {
		return Page{}, ErrUnavailable
	}
	return page, nil
}

// previewImageTypes are the image formats previews show.
var previewImageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true} //nolint:gochecknoglobals // constant lookup table

// Image fetches a preview's image, at most 5 MiB, for the browser to encrypt.
func (service *Service) Image(ctx context.Context, userKey string, raw string) (Image, error) {
	link, err := service.check(ctx, userKey, raw)
	if err != nil {
		return Image{}, err
	}
	body, mediaType, _, err := service.get(ctx, link, maximumImageBytes, func(mediaType string) bool { return previewImageTypes[mediaType] })
	if err != nil {
		return Image{}, err
	}
	if len(body) > maximumImageBytes {
		return Image{}, ErrUnavailable
	}
	return Image{Data: body, ContentType: mediaType}, nil
}
