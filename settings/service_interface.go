package settings

type Service interface {
	LoadSettings() error

	// GetSettings does not return error
	// because without settings Agent cannot start.
	GetSettings() Settings

	PublicSSHKeyForUsername(string) (string, error)

	InvalidateSettings() error
}
