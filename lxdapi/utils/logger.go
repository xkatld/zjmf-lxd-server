package utils

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func InitLogger(logFile string, maxSize, maxBackups, maxAge int, compress bool, level string, devMode bool) (*zap.Logger, error) {
	writer := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   compress,
	}
	
	var logLevel zapcore.Level
	switch level {
	case "debug":
		logLevel = zapcore.DebugLevel
	case "info":
		logLevel = zapcore.InfoLevel
	case "warn":
		logLevel = zapcore.WarnLevel
	case "error":
		logLevel = zapcore.ErrorLevel
	default:
		logLevel = zapcore.InfoLevel
	}
	
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeDuration = zapcore.MillisDurationEncoder
	encoderConfig.CallerKey = ""
	
	var consoleEncoder zapcore.Encoder
	if devMode {
		consoleEncoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		consoleEncoder = zapcore.NewJSONEncoder(encoderConfig)
	}
	
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
	
	consoleCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		logLevel,
	)
	
	fileCore := zapcore.NewCore(
		fileEncoder,
		zapcore.AddSync(writer),
		logLevel,
	)
	
	core := zapcore.NewTee(consoleCore, fileCore)
	
	logger := zap.New(core, zap.AddStacktrace(zapcore.ErrorLevel))
	
	return logger, nil
}

