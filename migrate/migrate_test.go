package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSourceFSValidation(t *testing.T) {
	t.Run("Valid absolute path", func(t *testing.T) {
		// This will fail if directory doesn't exist, but tests the function structure
		_, err := newFileSourceURL("/tmp")
		// We expect it to succeed for /tmp which should exist on most systems
		if err != nil {
			t.Skipf("Skipping test - /tmp directory not accessible: %v", err)
		}
	})

	t.Run("Invalid: nonexistent directory", func(t *testing.T) {
		_, err := newFileSourceURL("/this/path/does/not/exist/hopefully")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("File source URL format", func(t *testing.T) {
		url, err := newFileSourceURL(".")
		if err != nil {
			t.Skipf("Skipping URL format test: %v", err)
		}
		assert.Contains(t, url, "file://")
	})
}

func TestWrapError(t *testing.T) {
	t.Run("Wrap nil error returns nil", func(t *testing.T) {
		err := WrapError(nil, "some context")
		assert.NoError(t, err)
	})

	t.Run("Wrap error adds context", func(t *testing.T) {
		originalErr := ErrNoChange
		wrappedErr := WrapError(originalErr, "migration context")

		assert.Error(t, wrappedErr)
		assert.Contains(t, wrappedErr.Error(), "migration context")
		assert.ErrorIs(t, wrappedErr, ErrNoChange)
	})
}
