package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"lxdapi/config"
	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var (
	consoleTemplate *template.Template
	errorTemplate   *template.Template
)

func init() {
	consoleTemplate = template.Must(template.ParseFiles("templates/console.html"))
	errorTemplate = template.Must(template.ParseFiles("templates/console_error.html"))
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// ConsoleWebSocket WebSocket控制台连接（已废弃，建议使用token方式）
// @Summary WebSocket控制台连接
// @Tags Console
// @Param hostname query string true "容器名称"
// @Router /api/console/ws [get]
func ConsoleWebSocket(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname
	lc.Action = "console_ws"

	if hostname == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "缺少容器名称参数",
		})
		return
	}

	logger.Global.Info(ctx, "WebSocket控制台连接请求")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Global.Error(ctx, "WebSocket升级失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "WebSocket升级失败: " + err.Error(),
		})
		return
	}
	defer conn.Close()

	session, err := services.CreateConsoleSession(hostname, conn)
	if err != nil {
		logger.Global.Error(ctx, "创建控制台会话失败", zap.Error(err))
		conn.WriteMessage(websocket.TextMessage, []byte("错误: "+err.Error()+"\r\n"))
		return
	}
	defer session.Close()

	logger.Global.Info(ctx, "控制台会话建立成功", zap.String("session_id", session.ID))

	welcomeMsg := "=== LXD Web控制台 ===\r\n"
	welcomeMsg += "容器: " + hostname + "\r\n"
	welcomeMsg += "会话ID: " + session.ID + "\r\n"
	welcomeMsg += "========================\r\n"
	conn.WriteMessage(websocket.TextMessage, []byte(welcomeMsg))

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Global.Warn(ctx, "WebSocket连接异常关闭", zap.Error(err))
			} else {
				logger.Global.Info(ctx, "WebSocket连接正常关闭")
			}
			break
		}

		if messageType == websocket.TextMessage {
			var msg struct {
				Type string `json:"type"`
				Data string `json:"data"`
			}

			if err := json.Unmarshal(message, &msg); err != nil {
				if err := session.WriteInput(message); err != nil {
					logger.Global.Error(ctx, "写入控制台输入失败", zap.Error(err))
					conn.WriteMessage(websocket.TextMessage, []byte("错误: 写入失败\r\n"))
					break
				}
			} else if msg.Type == "input" {
				if err := session.WriteInput([]byte(msg.Data)); err != nil {
					logger.Global.Error(ctx, "写入控制台输入失败", zap.Error(err))
					conn.WriteMessage(websocket.TextMessage, []byte("错误: 写入失败\r\n"))
					break
				}
			}
		}
	}

	logger.Global.Info(ctx, "控制台会话结束", zap.String("session_id", session.ID))
}

// GetConsoleStatus 获取控制台连接状态
// @Summary 获取控制台状态
// @Tags Console
// @Param hostname query string true "容器名称"
// @Success 200 {object} map[string]interface{} "状态信息"
// @Router /api/console/status [get]
func GetConsoleStatus(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	containerName := c.Query("hostname")

	lc.Container = containerName

	if containerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "缺少容器名称参数",
		})
		return
	}

	containerInfo, err := services.GetContainerInfo(containerName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "success",
			"data": gin.H{
				"enabled":       true,
				"status":        "unavailable",
				"message":       "容器不存在或获取状态失败: " + err.Error(),
				"websocket_url": "",
			},
		})
		return
	}

	if containerInfo.Status != "Running" {
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "success",
			"data": gin.H{
				"enabled":       true,
				"status":        "stopped",
				"message":       "容器未运行，当前状态: " + containerInfo.Status,
				"websocket_url": "",
			},
		})
		return
	}

	scheme := "ws"
	if c.Request.TLS != nil {
		scheme = "wss"
	}
	websocketURL := scheme + "://" + c.Request.Host + "/api/console/ws?hostname=" + containerName

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": gin.H{
			"enabled":       true,
			"status":        "available",
			"message":       "控制台可用",
			"websocket_url": websocketURL,
			"container": gin.H{
				"name":   containerInfo.Name,
				"status": containerInfo.Status,
			},
		},
	})
}

// ListConsoleSessions 获取控制台会话列表
// @Summary 获取会话列表
// @Tags Console
// @Success 200 {object} map[string]interface{} "会话列表"
// @Router /api/console/sessions [get]
func ListConsoleSessions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": gin.H{
			"enabled": true,
			"timeout": config.AppConfig.ConsoleAPI.SessionTimeout,
			"message": "控制台会话管理接口",
		},
	})
}

// CloseConsoleSession 关闭控制台会话
// @Summary 关闭会话
// @Tags Console
// @Param sessionId path string true "会话ID"
// @Success 200 {object} map[string]interface{} "关闭结果"
// @Router /api/console/sessions/{sessionId} [delete]
func CloseConsoleSession(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := c.Param("sessionId")

	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "缺少会话ID参数",
		})
		return
	}

	session, exists := services.GetConsoleSession(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "会话不存在",
		})
		return
	}

	go session.Close()

	logger.Global.Info(ctx, "管理员关闭控制台会话", zap.String("session_id", sessionID))

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "会话已关闭",
		"data": gin.H{
			"session_id": sessionID,
		},
	})
}

// CreateConsoleToken 创建控制台访问令牌用于安全连接
// @Summary 创建控制台访问令牌
// @Description 创建用于WebSocket控制台连接的安全令牌
// @Tags Web控制台
// @Accept json
// @Produce json
// @Param data body models.CreateTokenRequest true "令牌参数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /api/console/create-token [post]
func CreateConsoleToken(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "创建控制台令牌请求")

	var req models.CreateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Global.Error(ctx, "参数解析失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误: " + err.Error(),
		})
		return
	}

	lc.Container = req.Hostname
	lc.Action = "create_console_token"

	if req.ExpiresIn <= 0 {
		req.ExpiresIn = 3600
	}

	logger.Global.Info(ctx, "创建控制台令牌参数",
		zap.String("server_ip", req.ServerIP),
		zap.Int("user_id", req.UserID),
		zap.Int("service_id", req.ServiceID),
		zap.Int("expires_in", req.ExpiresIn))

	tokenResp, err := services.GenerateConsoleToken(req.Hostname, req.ServerIP, req.UserID, req.ServiceID, req.ExpiresIn)
	if err != nil {
		logger.Global.Error(ctx, "创建控制台令牌失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "生成令牌失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "创建控制台令牌成功", zap.String("token", tokenResp.Token))

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": tokenResp,
	})
}

// ConsoleWebSocketWithToken 使用令牌进行WebSocket控制台连接
// @Summary 基于令牌的WebSocket控制台
// @Tags Web控制台
// @Param token query string true "访问令牌"
// @Router /api/console/ws-token [get]
func ConsoleWebSocketWithToken(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	token := c.Query("token")

	lc.Action = "console_ws_token"

	logger.Global.Info(ctx, "WebSocket控制台连接请求", zap.String("token", token))

	if token == "" {
		logger.Global.Warn(ctx, "缺少令牌参数")
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "缺少令牌参数",
		})
		return
	}

	consoleToken, err := services.ValidateConsoleToken(token)
	if err != nil {
		logger.Global.Error(ctx, "令牌验证失败", zap.String("token", token), zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "令牌无效: " + err.Error(),
		})
		return
	}

	lc.Container = consoleToken.Hostname

	logger.Global.Info(ctx, "令牌验证成功")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Global.Error(ctx, "WebSocket升级失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "WebSocket升级失败: " + err.Error(),
		})
		return
	}
	defer conn.Close()

	services.MarkTokenAsUsed(token)

	session, err := services.CreateConsoleSession(consoleToken.Hostname, conn)
	if err != nil {
		logger.Global.Error(ctx, "创建控制台会话失败", zap.Error(err))
		conn.WriteMessage(websocket.TextMessage, []byte("错误: "+err.Error()+"\r\n"))
		return
	}
	defer session.Close()

	logger.Global.Info(ctx, "令牌控制台会话建立成功", zap.String("session_id", session.ID))

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Global.Warn(ctx, "令牌WebSocket连接异常关闭", zap.Error(err))
			} else {
				logger.Global.Info(ctx, "令牌WebSocket连接正常关闭")
			}
			break
		}

		if messageType == websocket.TextMessage {
			var msg struct {
				Type string `json:"type"`
				Data string `json:"data"`
			}

			if err := json.Unmarshal(message, &msg); err != nil {
				if err := session.WriteInput(message); err != nil {
					logger.Global.Error(ctx, "写入控制台输入失败", zap.Error(err))
					conn.WriteMessage(websocket.TextMessage, []byte("错误: 写入失败\r\n"))
					break
				}
			} else if msg.Type == "input" {
				if err := session.WriteInput([]byte(msg.Data)); err != nil {
					logger.Global.Error(ctx, "写入控制台输入失败", zap.Error(err))
					conn.WriteMessage(websocket.TextMessage, []byte("错误: 写入失败\r\n"))
					break
				}
			}
		}
	}

	logger.Global.Info(ctx, "令牌控制台会话结束", zap.String("session_id", session.ID))
}

// ConsolePageHandler 控制台页面处理器提供Web界面
// @Summary 控制台Web页面
// @Tags Web控制台
// @Param token query string true "访问令牌"
// @Produce html
// @Router /console [get]
func ConsolePageHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	token := c.Query("token")

	logger.Global.Info(ctx, "控制台页面访问请求", zap.String("token", token))

	if token == "" {
		logger.Global.Warn(ctx, "缺少令牌参数")
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusBadRequest)
		errorTemplate.Execute(c.Writer, gin.H{"Message": "缺少访问令牌，请通过正确的链接访问控制台。"})
		return
	}

	var consoleToken models.ConsoleToken
	currentTime := time.Now()

	logger.Global.Debug(ctx, "开始验证令牌", zap.Time("current_time", currentTime))

	if err := database.DB.Where("token = ? AND used = false AND expires_at > ?", token, currentTime).First(&consoleToken).Error; err != nil {
		logger.Global.Error(ctx, "令牌验证失败", zap.Error(err))

		var existingToken models.ConsoleToken
		if err2 := database.DB.Where("token = ?", token).First(&existingToken).Error; err2 == nil {
			logger.Global.Debug(ctx, "找到令牌但验证失败",
				zap.Bool("used", existingToken.Used),
				zap.Time("expires_at", existingToken.ExpiresAt))
		} else {
			logger.Global.Debug(ctx, "令牌不存在")
		}

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Status(http.StatusUnauthorized)
		errorTemplate.Execute(c.Writer, gin.H{"Message": "访问令牌无效或已过期，请重新获取访问链接。"})
		return
	}

	lc.Container = consoleToken.Hostname

	logger.Global.Info(ctx, "控制台页面令牌验证成功", zap.Int("user_id", consoleToken.UserID))

	scheme := "ws"
	if c.Request.TLS != nil {
		scheme = "wss"
	}
	wsUrl := scheme + "://" + c.Request.Host + "/api/console/ws-token?token=" + token

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(http.StatusOK)
	consoleTemplate.Execute(c.Writer, gin.H{
		"Hostname": consoleToken.Hostname,
		"Token":    token,
		"WSUrl":    wsUrl,
	})
}
