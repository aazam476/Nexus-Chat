package contract

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstanceInfoIsPublic(t *testing.T) {
	server := setUpInstance(t)

	info := server.call(t, http.MethodGet, "/api/v1/instance", "", http.StatusOK)

	assert.Contains(t, string(info), `"registrationMode":"invite_only"`)
	assert.Contains(t, string(info), `"secureConnection":true`, "127.0.0.1 counts as localhost")
}

func TestAdminTLSEndpoints(t *testing.T) {
	server := setUpInstanceWithDomain(t)
	admin := server.login(t, "Laptop")

	status := server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/admin/tls", http.StatusOK)
	assert.JSONEq(t, `{"active":false}`, string(status))

	upload := requestSpec{method: http.MethodPut, path: "/api/v1/admin/tls", body: `{"certificate": "not a certificate", "privateKey": "not a key"}`}
	rejected := server.authorizedCallWithBody(t, admin.Token, upload, http.StatusUnprocessableEntity)
	assert.Contains(t, string(rejected), `"code":"certificate_rejected"`)

	server.authorizedCall(t, admin.Token, http.MethodDelete, "/api/v1/admin/tls", http.StatusNoContent)
}

func TestAdminSMTPEndpoints(t *testing.T) {
	server := setUpInstance(t)
	admin := server.login(t, "Laptop")

	initial := server.authorizedCall(t, admin.Token, http.MethodGet, "/api/v1/admin/smtp", http.StatusOK)
	assert.Contains(t, string(initial), `"configured":false`)
	notConfigured := server.authorizedCall(t, admin.Token, http.MethodPost, "/api/v1/admin/smtp/test", http.StatusUnprocessableEntity)
	assert.Contains(t, string(notConfigured), `"code":"smtp_not_configured"`)

	invalid := requestSpec{method: http.MethodPut, path: "/api/v1/admin/smtp", body: `{"host": "", "port": 0, "security": "tls", "from": "x"}`}
	server.authorizedCallWithBody(t, admin.Token, invalid, http.StatusUnprocessableEntity)

	// Nothing listens on this port, so the test email cannot be delivered.
	body := fmt.Sprintf(`{"host": "127.0.0.1", "port": %d, "security": "none", "from": "chat@example.com"}`, freePort(t))
	saved := server.authorizedCallWithBody(t, admin.Token, requestSpec{method: http.MethodPut, path: "/api/v1/admin/smtp", body: body}, http.StatusOK)
	assert.Contains(t, string(saved), `"configured":true`)
	assert.Contains(t, string(saved), `"passwordSet":false`)

	failed := server.authorizedCall(t, admin.Token, http.MethodPost, "/api/v1/admin/smtp/test", http.StatusBadGateway)
	assert.Contains(t, string(failed), `"code":"mail_delivery_failed"`)
}
