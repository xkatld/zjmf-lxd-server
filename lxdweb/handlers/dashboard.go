package handlers
import (
	"net/http"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)
// DashboardPage 仪表盘页面
// @Summary 仪表盘页面
// @Description 显示系统仪表盘页面
// @Tags 仪表盘
// @Produce html
// @Success 200 {string} string "HTML页面"
// @Router / [get]
func DashboardPage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":    "仪表盘 - LXD管理后台",
		"username": username,
	})
}
