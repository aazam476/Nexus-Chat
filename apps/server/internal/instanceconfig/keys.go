package instanceconfig

// Keys of instance settings. Every feature reads and writes settings
// through these constants, so a key is spelled in exactly one place.
const (
	// KeyInstanceName is the instance's display name (JSON string).
	KeyInstanceName = "instance.name"
	// KeyInstanceDomain is the public domain set in the setup wizard or
	// Admin panel (JSON string). DOMAIN in .env takes precedence.
	KeyInstanceDomain = "instance.domain"
	// KeyRegistrationMode is "open", "invite_only" or "closed" (JSON string).
	KeyRegistrationMode = "registration.mode"
	// KeySetupCompletedAt is when setup finished (JSON timestamp). Its
	// presence is what marks setup as complete.
	KeySetupCompletedAt = "setup.completed_at"
	// KeySMTP is the mail server configuration (JSON mail.SMTPConfig).
	KeySMTP = "smtp"
	// KeyAttachmentMaxBytes is the instance's attachment limit in bytes
	// (JSON number), set in the Admin panel.
	KeyAttachmentMaxBytes = "uploads.attachment_max_bytes"
	// KeyLinkPreviewProxy turns the web app's link preview proxy on
	// (JSON boolean); off unless the admin turns it on.
	KeyLinkPreviewProxy = "previews.proxy_enabled"
)
