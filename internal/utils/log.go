package utils

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *zap.SugaredLogger

func init() {
	config := zap.NewDevelopmentConfig()
	config.Encoding = "console"

	//config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	globalLogger = logger.Sugar()
}

// GetLogger 패키지별 로거를 반환합니다
func GetLogger() *zap.SugaredLogger {
	return globalLogger
}

// Cleanup 로거 정리
func Cleanup() {
	if globalLogger != nil {
		err := globalLogger.Sync()
		if err != nil {
			return
		}
	}
}
