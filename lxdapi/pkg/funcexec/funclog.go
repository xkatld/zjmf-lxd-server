package funcexec

import (
	"context"
	"time"

	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

type FuncLog struct {
	ctx      context.Context
	funcName string
	start    time.Time
}

func newFuncLog(ctx context.Context, funcName string) *FuncLog {
	return &FuncLog{
		ctx:      ctx,
		funcName: funcName,
		start:    time.Now(),
	}
}

func (fl *FuncLog) Debug(msg string, fields ...zap.Field) {
	fields = append(fields, zap.String("func", fl.funcName))
	logger.Global.Debug(fl.ctx, msg, fields...)
}

func (fl *FuncLog) Info(msg string, fields ...zap.Field) {
	fields = append(fields, zap.String("func", fl.funcName))
	logger.Global.Info(fl.ctx, msg, fields...)
}

func (fl *FuncLog) Warn(msg string, fields ...zap.Field) {
	fields = append(fields, zap.String("func", fl.funcName))
	logger.Global.Warn(fl.ctx, msg, fields...)
}

func (fl *FuncLog) Error(msg string, fields ...zap.Field) {
	fields = append(fields, zap.String("func", fl.funcName))
	logger.Global.Error(fl.ctx, msg, fields...)
}

