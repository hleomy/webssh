package handler

import (
	"github.com/google/uuid"
)

func parseUUID(s string) uuid.UUID {
	id, _ := uuid.Parse(s)
	return id
}
