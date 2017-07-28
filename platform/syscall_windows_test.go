// +build windows

package platform

// Export for test cleanup
var DeleteUserProfile = deleteUserProfile

// Export for testing
var (
	UserHomeDirectory    = userHomeDirectory
	RandomPassword       = randomPassword
	ValidWindowsPassword = validPassword
	LocalAccountNames    = localAccountNames
)
