package must

import "fmt"

// Package is inspired by https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md
// "Assertions detect programmer errors. Unlike operating errors, which are expected and which must be handled, assertion failures are unexpected.
// The only correct way to handle corrupt code is to crash. Assertions downgrade catastrophic correctness bugs into liveness bugs.
// Assertions are a force multiplier for discovering bugs by fuzzing."

// Panics are useful as they provide a stack trace and faster recovery (MTTR).

// Succeed panics on error.
func Succeed[T any](obj T, err error) T {
	if err != nil {
		panic(fmt.Errorf("assertion failed: %w", err))
	}
	return obj
}

// BeTrue panics if value is false
func BeTrue(value bool) {
	if !value {
		panic(fmt.Sprintf("assertion failed: expected %t to be true", value))
	}
}

// BeFalse panics if value is true
func BeFalse(value bool) {
	if value {
		panic(fmt.Sprintf("assertion failed: expected %t to be false", value))
	}
}
