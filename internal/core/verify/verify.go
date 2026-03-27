package verify

import "context"

// Verifier performs post-render validation and reload actions.
type Verifier interface {
	ConfigTest(ctx context.Context) error
	Reload(ctx context.Context) error
	Restart(ctx context.Context) error
}
