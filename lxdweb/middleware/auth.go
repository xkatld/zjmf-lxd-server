package middleware
import (
	"net/http"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		adminID := session.Get("admin_id")
		if adminID == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Set("admin_id", adminID)
		c.Next()
	}
}
