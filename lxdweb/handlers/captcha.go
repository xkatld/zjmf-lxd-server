package handlers
import (
	"net/http"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
)
var store = base64Captcha.DefaultMemStore
// GetCaptcha 获取验证码
// @Summary 获取验证码
// @Description 生成登录验证码图片
// @Tags 认证管理
// @Produce json
// @Success 200 {object} map[string]interface{} "成功返回验证码"
// @Failure 500 {object} map[string]interface{} "生成失败"
// @Router /api/captcha [get]
func GetCaptcha(c *gin.Context) {
	driver := base64Captcha.NewDriverDigit(80, 240, 4, 0.7, 80)
	captcha := base64Captcha.NewCaptcha(driver, store)
	id, b64s, _, err := captcha.Generate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "生成验证码失败",
		})
		return
	}
	session := sessions.Default(c)
	session.Set("captcha_id", id)
	session.Save()
	c.JSON(http.StatusOK, gin.H{
		"code":       200,
		"captcha_id": id,
		"image":      b64s,
	})
}
func VerifyCaptcha(captchaID, captchaValue string) bool {
	return store.Verify(captchaID, captchaValue, true)
}
