package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var Logger *zap.SugaredLogger

func init() {
	env := os.Getenv("APP_ENV") // 환경변수로 개발/운영 분기 (예: "development" or "production")

	var logger *zap.Logger
	var err error

	if env == "prod" {
		logger, err = zap.NewProduction()
	} else {
		config := zap.NewDevelopmentConfig()
		config.Encoding = "console"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		logger, err = config.Build()
	}

	if err != nil {
		panic(err)
	}

	Logger = logger.Sugar()
}

// 필요시 래핑 함수도 추가
func Infof(format string, args ...interface{})  { Logger.Infof(format, args...) }
func Errorf(format string, args ...interface{}) { Logger.Errorf(format, args...) }
func Warnf(format string, args ...interface{})  { Logger.Warnf(format, args...) }
func Debugf(format string, args ...interface{}) { Logger.Debugf(format, args...) }
