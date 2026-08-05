package logger

import "go.uber.org/zap"

var Global *Logger

func Init(zl *zap.Logger) {
	Global = New(zl)
}

