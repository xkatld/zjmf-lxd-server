package handlers
import (
	"lxdweb/database"
	"lxdweb/models"
	"net/http"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)
// LoginPage 登录页面
// @Summary 登录页面
// @Description 显示登录页面
// @Tags 认证管理
// @Produce html
// @Success 200 {string} string "HTML页面"
// @Router /login [get]
func LoginPage(c *gin.Context) {
	session := sessions.Default(c)
	if adminID := session.Get("admin_id"); adminID != nil {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "登录 - LXD管理后台",
	})
}
// Login 用户登录
// @Summary 用户登录
// @Description 验证用户名、密码和验证码，登录成功后创建会话
// @Tags 认证管理
// @Accept json
// @Produce json
// @Param body body object true "登录参数(username, password, captcha)"
// @Success 200 {object} map[string]interface{} "登录成功"
// @Failure 400 {object} map[string]interface{} "参数错误或验证码错误"
// @Failure 401 {object} map[string]interface{} "用户名或密码错误"
// @Router /api/login [post]
func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" form:"username" binding:"required"`
		Password string `json:"password" form:"password" binding:"required"`
		Captcha  string `json:"captcha" form:"captcha" binding:"required"`
	}
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "请填写完整信息",
		})
		return
	}
	session := sessions.Default(c)
	captchaID := session.Get("captcha_id")
	if captchaID == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "验证码已过期，请刷新",
		})
		return
	}
	if !VerifyCaptcha(captchaID.(string), req.Captcha) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "验证码错误",
		})
		return
	}
	session.Delete("captcha_id")
	session.Save()
	var admin models.Admin
	if err := database.DB.Where("username = ?", req.Username).First(&admin).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "用户名或密码错误",
		})
		return
	}
	if !admin.CheckPassword(req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "用户名或密码错误",
		})
		return
	}
	session.Set("admin_id", admin.ID)
	session.Set("username", admin.Username)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "登录失败",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "登录成功",
		"data": gin.H{
			"redirect": "/dashboard",
		},
	})
}
// Logout 用户登出
// @Summary 用户登出
// @Description 清除用户会话，退出登录
// @Tags 认证管理
// @Produce json
// @Success 200 {object} map[string]interface{} "退出成功"
// @Router /api/logout [post]
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/login")
}
