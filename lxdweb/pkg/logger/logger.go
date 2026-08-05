package logger

import (
	"context"
	"runtime"
	"strings"

	"go.uber.org/zap"
)

type Logger struct {
	*zap.Logger
}

func New(zl *zap.Logger) *Logger {
	return &Logger{Logger: zl}
}

func (l *Logger) withContext(ctx context.Context, fields []zap.Field) []zap.Field {
	lc := FromContext(ctx)
	
	caller := getCaller(3)
	
	contextFields := []zap.Field{
		zap.String("trace_id", lc.TraceID),
		zap.String("caller", caller),
	}
	
	if lc.NodeID > 0 {
		contextFields = append(contextFields, zap.Uint("node_id", lc.NodeID))
	}
	if lc.Container != "" {
		contextFields = append(contextFields, zap.String("container", lc.Container))
	}
	if lc.Action != "" {
		contextFields = append(contextFields, zap.String("action", lc.Action))
	}
	if lc.Username != "" {
		contextFields = append(contextFields, zap.String("username", lc.Username))
	}
	
	return append(contextFields, fields...)
}

func (l *Logger) Debug(ctx context.Context, msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, l.withContext(ctx, fields)...)
}

func (l *Logger) Info(ctx context.Context, msg string, fields ...zap.Field) {
	l.Logger.Info(msg, l.withContext(ctx, fields)...)
}

func (l *Logger) Warn(ctx context.Context, msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, l.withContext(ctx, fields)...)
}

func (l *Logger) Error(ctx context.Context, msg string, fields ...zap.Field) {
	l.Logger.Error(msg, l.withContext(ctx, fields)...)
}

func getCaller(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}
	
	fullName := fn.Name()
	parts := strings.Split(fullName, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if idx := strings.LastIndex(name, "."); idx != -1 {
			return name[idx+1:]
		}
		return name
	}
	
	return fullName
}

