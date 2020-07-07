package util

import (
	"context"
)

// RunnerWithContext defines API contract for implementors that
// start and block in processing until a stop signal is received
// from the context. A tipical use is in systems with multiple
// concurent runners that share context.
// Typical implementors would be servers and server-like programs.
type RunnerWithContext interface {
	// Run blocks in processing until context sends a stop signal
	// because it has been interrupted externally. It can be cancelled
	// also by this function to broadcast stop to other parallel
	// routines.
	// Run should change accordingly the implementation running state
	// so that IsRunning can be used to query for it.
	Run(ctx context.Context, cancel context.CancelFunc) error
	// IsRunning returns the running state of this RunnerWithContext
	IsRunning() bool
}
