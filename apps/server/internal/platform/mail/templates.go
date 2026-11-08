package mail

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

// templateFiles holds every email the server sends. Keeping the wording in
// files, not in Go code, is what lets the texts be translated later
// (spec §4.13). The first line of each file is the subject.
//
//go:embed templates/*.txt
var templateFiles embed.FS

// Template names.
const (
	TemplateVerifyEmail   = "verify-email"
	TemplateResetPassword = "reset-password"
	TemplateTestEmail     = "test-email"
	// TemplateCertificateExpiring warns the admin before HTTPS breaks.
	TemplateCertificateExpiring = "certificate-expiring"
)

// TemplateData fills in a template.
type TemplateData struct {
	InstanceName string
	DisplayName  string
	Link         string
	// For certificate warnings: the domain, the expiry date and the time
	// left in words ("7 days").
	Domain   string
	Expires  string
	DaysLeft string
}

// Render builds the message for template name addressed to to.
func Render(name string, to string, data TemplateData) (Message, error) {
	parsed, err := template.ParseFS(templateFiles, "templates/"+name+".txt")
	if err != nil {
		return Message{}, fmt.Errorf("loading email template %s: %w", name, err)
	}
	var output bytes.Buffer
	if err := parsed.Execute(&output, data); err != nil {
		return Message{}, fmt.Errorf("filling in email template %s: %w", name, err)
	}
	subject, body, _ := strings.Cut(output.String(), "\n")
	return Message{To: to, Subject: strings.TrimSpace(subject), Text: strings.TrimLeft(body, "\n")}, nil
}
