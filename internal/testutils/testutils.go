// Package testutils contains utilities for testing.
package testutils

import (
	"fmt"
	"testing"
)

// AssertError returns a message if got error is not as wanted.
func AssertError(t *testing.T, got error, wantError bool) string {
	t.Helper()

	if wantError && got == nil {
		return "want an error, got nil"
	}
	if !wantError && got != nil {
		return fmt.Sprintf("want no error, got %v", got)
	}

	return ""
}
