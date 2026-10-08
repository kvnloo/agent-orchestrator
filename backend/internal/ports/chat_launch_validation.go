package ports

// ChatLaunchValidator is optionally implemented by drivers whose launch-time
// settings have provider-specific constraints that capability bits cannot
// express. Validation runs during spawn preflight, before AO creates durable
// session or workspace state.
type ChatLaunchValidator interface {
	ValidateLaunch(permissions PermissionMode) error
}
