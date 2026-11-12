package contract

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheLinkPreviewProxy(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")
	bob := server.newUser(t, admin.Token, "bob")
	preview := func(path string, link string, want int) []byte {
		return server.authorizedCall(t, bob.Token, http.MethodGet, path+"?url="+url.QueryEscape(link), want)
	}

	assert.Contains(t, string(preview("/api/v1/link-previews", "https://example.com/", http.StatusForbidden)), "preview_proxy_off")
	assert.Contains(t, string(server.call(t, http.MethodGet, "/api/v1/instance", "", http.StatusOK)), `"linkPreviewProxy":false`)
	assert.JSONEq(t, `{"proxyEnabled": false}`, string(server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/admin/previews", http.StatusOK)))
	server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPut, path: "/api/v1/admin/previews", body: `{"proxyEnabled": true}`}, http.StatusOK)
	server.authorizedCallWithBody(t, bob.Token, requestSpec{method: http.MethodPut, path: "/api/v1/admin/previews", body: `{"proxyEnabled": false}`}, http.StatusForbidden)

	assert.Contains(t, string(preview("/api/v1/link-previews", "file:///etc/passwd", http.StatusUnprocessableEntity)), "invalid_link")
	// The instance itself, and anything internal, is never fetched.
	assert.Contains(t, string(preview("/api/v1/link-previews", server.baseURL+"/api/v1/instance", http.StatusBadGateway)), "preview_unavailable")
	assert.Contains(t, string(preview("/api/v1/link-previews/image", "http://169.254.169.254/latest/meta-data", http.StatusBadGateway)), "preview_unavailable")
	assert.Contains(t, string(preview("/api/v1/link-previews/image", "gopher://x", http.StatusUnprocessableEntity)), "invalid_link")
	assert.Contains(t, string(server.call(t, http.MethodGet, "/api/v1/instance", "", http.StatusOK)), `"linkPreviewProxy":true`)
}
