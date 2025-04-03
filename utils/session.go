package utils

import (
	"fmt"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func CheckSessionStore(c *gin.Context) error {
	session := sessions.Default(c)
	session.Set("test", "test")
	if err := session.Save(); err != nil {
		return fmt.Errorf("session store check failed: %v", err)
	}
	session.Delete("test")
	return session.Save()
}
