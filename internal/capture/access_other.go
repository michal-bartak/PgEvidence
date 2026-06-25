//go:build !darwin

package capture

// HasScreenAccess is always true off macOS, where screenshots need no special
// per-app TCC permission.
func HasScreenAccess() bool { return true }

// RequestScreenAccess is a no-op off macOS.
func RequestScreenAccess() bool { return true }
