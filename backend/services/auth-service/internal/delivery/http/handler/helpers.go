package handler

import (
	"github.com/google/uuid"
)

// parseUUID is the real implementation that replaces the stub.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
