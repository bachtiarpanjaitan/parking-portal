package users

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// uuidFromParam parses :id (or :name) from the gin context into a UUID.
func uuidFromParam(c *gin.Context, name string) (uuid.UUID, error) {
	return uuid.Parse(c.Param(name))
}
