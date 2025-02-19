package must

import "fmt"

// Succeed should only be used in applications when we are absolutely sure that the function would not throw an error
// and if we receive an error we can not recover from it.
// It should not be used by libraries as panicing there could cause an unexpected break of the app using the library.
// Panics are useful as they provide a stack trace and broken unrecoverable invariants are instantly discoverable.
// This optimizes for fast recovery (MTTR)
func Succeed[T any](obj T, err error) T {
	if err != nil {
		panic(fmt.Errorf("invariant broken: %w", err))
	}
	return obj
}
