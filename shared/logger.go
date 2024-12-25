package shared

import (
	"sync"

	tunnelConfig "github.com/niradler/go-netbridge/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
	once   sync.Once
)

func InitLogger(opt tunnelConfig.Config) {
	once.Do(func() {
		var err error

		var config zap.Config
		if opt.LOG_JSON {
			// JSON logging
			config = zap.NewProductionConfig()
		} else {
			// Human-readable logging (structured)
			config = zap.NewDevelopmentConfig()
		}

		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.MessageKey = "message"
		config.EncoderConfig.LevelKey = "level"
		config.EncoderConfig.CallerKey = "caller"
		config.EncoderConfig.StacktraceKey = "stacktrace"

		if opt.LOG_FILE != "" {
			config.OutputPaths = []string{opt.LOG_FILE}
		} else {
			config.OutputPaths = []string{"stdout"}
		}

		level, err := zapcore.ParseLevel(opt.LOG_LEVEL)
		if err == nil {
			config.Level.SetLevel(level)
		}

		logger, err = config.Build()
		if err != nil {
			panic("Failed to initialize logger: " + err.Error())
		}
	})
}

func GetLogger() *zap.Logger {
	if logger == nil {
		panic("Failed to initialize logger before used")
	}
	return logger
}
