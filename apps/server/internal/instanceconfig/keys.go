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
)
