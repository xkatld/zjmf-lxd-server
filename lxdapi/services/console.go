package services

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"lxdapi/config"
	"lxdapi/pkg/logger"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type ConsoleSession struct {
	ID          string
	ContainerID string
	Conn        *websocket.Conn
	Cmd         *exec.Cmd
	StdinPipe   io.WriteCloser
	StdoutPipe  io.ReadCloser
	StderrPipe  io.ReadCloser
	Cancel      context.CancelFunc
	LastActive  time.Time
	Mutex       sync.Mutex
}

type ConsoleManager struct {
	sessions map[string]*ConsoleSession
	mutex    sync.RWMutex
}

var consoleManager = &ConsoleManager{
	sessions: make(map[string]*ConsoleSession),
}

func CreateConsoleSession(containerName string, conn *websocket.Conn) (*ConsoleSession, error) {
	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "create_console_session",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "创建控制台会话")

	status, err := GetContainerStatus(containerName)
	if err != nil {
		logger.Global.Error(ctx, "获取容器状态失败", zap.Error(err))
		return nil, fmt.Errorf("容器 %s 未运行或不存在: %v", containerName, err)
	}

	logger.Global.Debug(ctx, "容器状态", zap.String("status", status))
	if status != "Running" {
		return nil, fmt.Errorf("容器 %s 状态为 %s，需要运行状态才能连接控制台", containerName, status)
	}

	sessionID := fmt.Sprintf("%s_%d", containerName, time.Now().Unix())

	ctx, cancel := context.WithCancel(context.Background())

	// 尝试使用 bash，如果失败则使用 sh（Alpine 等轻量级镜像）
	shell := "/bin/bash"
	checkCmd := exec.Command("lxc", "exec", containerName, "--", "test", "-f", "/bin/bash")
	if err := checkCmd.Run(); err != nil {
		shell = "/bin/sh"
		logger.Global.Debug(ctx, "容器不支持 bash，使用 sh", zap.String("shell", shell))
	}

	cmd := exec.CommandContext(ctx, "lxc", "exec", containerName,
		"-t",
		"--env", "TERM=xterm-256color",
		"--env", "COLORTERM=truecolor",
		"--", shell, "-i")

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("创建stdin管道失败: %v", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		cancel()
		return nil, fmt.Errorf("创建stdout管道失败: %v", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		cancel()
		return nil, fmt.Errorf("创建stderr管道失败: %v", err)
	}

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		cancel()
		return nil, fmt.Errorf("启动控制台命令失败: %v", err)
	}

	session := &ConsoleSession{
		ID:          sessionID,
		ContainerID: containerName,
		Conn:        conn,
		Cmd:         cmd,
		StdinPipe:   stdinPipe,
		StdoutPipe:  stdoutPipe,
		StderrPipe:  stderrPipe,
		Cancel:      cancel,
		LastActive:  time.Now(),
	}

	consoleManager.mutex.Lock()
	consoleManager.sessions[sessionID] = session
	consoleManager.mutex.Unlock()

	logger.Global.Info(ctx, "控制台会话创建成功", zap.String("session_id", sessionID))

	go session.handleOutput()
	go session.handleError()

	return session, nil
}

func (s *ConsoleSession) handleOutput() {
	ctx := context.Background()
	lc := &logger.Context{
		Container: s.ContainerID,
		Action:    "console_output",
	}
	ctx = logger.NewContext(ctx, lc)

	defer func() {
		if r := recover(); r != nil {
			logger.Global.Error(ctx, "控制台输出处理异常", zap.Any("panic", r))
		}
	}()

	buffer := make([]byte, 4096)
	for {
		n, err := s.StdoutPipe.Read(buffer)
		if err != nil {
			if err != io.EOF {
				logger.Global.Debug(ctx, "读取控制台输出错误", zap.Error(err))
			}
			break
		}

		if n > 0 {
			s.Mutex.Lock()
			if s.Conn != nil {
				s.LastActive = time.Now()
				if err := s.Conn.WriteMessage(websocket.TextMessage, buffer[:n]); err != nil {
					logger.Global.Debug(ctx, "发送输出到WebSocket失败", zap.Error(err))
					s.Mutex.Unlock()
					break
				}
			}
			s.Mutex.Unlock()
		}
	}
}

func (s *ConsoleSession) handleError() {
	ctx := context.Background()
	lc := &logger.Context{
		Container: s.ContainerID,
		Action:    "console_error",
	}
	ctx = logger.NewContext(ctx, lc)

	defer func() {
		if r := recover(); r != nil {
			logger.Global.Error(ctx, "控制台错误处理异常", zap.Any("panic", r))
		}
	}()

	buffer := make([]byte, 4096)
	for {
		n, err := s.StderrPipe.Read(buffer)
		if err != nil {
			if err != io.EOF {
				logger.Global.Debug(ctx, "读取控制台错误输出错误", zap.Error(err))
			}
			break
		}

		if n > 0 {
			s.Mutex.Lock()
			if s.Conn != nil {
				s.LastActive = time.Now()
				if err := s.Conn.WriteMessage(websocket.TextMessage, buffer[:n]); err != nil {
					logger.Global.Debug(ctx, "发送错误输出到WebSocket失败", zap.Error(err))
					s.Mutex.Unlock()
					break
				}
			}
			s.Mutex.Unlock()
		}
	}
}

func (s *ConsoleSession) WriteInput(data []byte) error {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	if s.StdinPipe == nil {
		return fmt.Errorf("stdin管道已关闭")
	}

	s.LastActive = time.Now()
	_, err := s.StdinPipe.Write(data)
	if err != nil {
		ctx := context.Background()
		lc := &logger.Context{
			Container: s.ContainerID,
			Action:    "console_input",
		}
		ctx = logger.NewContext(ctx, lc)
		logger.Global.Error(ctx, "写入控制台输入失败", zap.Error(err))
		return err
	}

	if f, ok := s.StdinPipe.(interface{ Sync() error }); ok {
		f.Sync()
	}

	return nil
}

func (s *ConsoleSession) Close() {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	ctx := context.Background()
	lc := &logger.Context{
		Container: s.ContainerID,
		Action:    "close_console_session",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "关闭控制台会话", zap.String("session_id", s.ID))

	if s.Conn != nil {
		s.Conn.Close()
		s.Conn = nil
	}

	if s.StdinPipe != nil {
		s.StdinPipe.Close()
		s.StdinPipe = nil
	}
	if s.StdoutPipe != nil {
		s.StdoutPipe.Close()
		s.StdoutPipe = nil
	}
	if s.StderrPipe != nil {
		s.StderrPipe.Close()
		s.StderrPipe = nil
	}

	if s.Cancel != nil {
		s.Cancel()
	}

	if s.Cmd != nil && s.Cmd.Process != nil {
		s.Cmd.Process.Kill()
		s.Cmd.Wait()
	}

	consoleManager.mutex.Lock()
	delete(consoleManager.sessions, s.ID)
	consoleManager.mutex.Unlock()
}

func GetConsoleSession(sessionID string) (*ConsoleSession, bool) {
	consoleManager.mutex.RLock()
	defer consoleManager.mutex.RUnlock()

	session, exists := consoleManager.sessions[sessionID]
	return session, exists
}

func CleanupExpiredSessions() {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "cleanup_console_sessions",
	}
	ctx = logger.NewContext(ctx, lc)

	consoleManager.mutex.Lock()
	defer consoleManager.mutex.Unlock()

	timeout := time.Duration(config.AppConfig.ConsoleAPI.SessionTimeout) * time.Second
	now := time.Now()

	for sessionID, session := range consoleManager.sessions {
		if now.Sub(session.LastActive) > timeout {
			logger.Global.Info(ctx, "清理过期控制台会话", zap.String("session_id", sessionID))
			go session.Close()
		}
	}
}

func StartConsoleCleanupTask() {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "console_cleanup_task",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "启动控制台会话清理任务")

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			CleanupExpiredSessions()
		}
	}()
}
