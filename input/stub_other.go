//go:build !linux

package input

// NewBackend returns a no-op backend on platforms without a global input
// monitor yet. The Backend interface keeps the activity layer portable so
// macOS (event taps) / Windows (low-level hooks) backends can be added later
// without touching the game loop.
func NewBackend() Backend {
	return NewNone()
}
