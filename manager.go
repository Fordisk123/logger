package logger

import (
	"context"
	"go.uber.org/zap/zapcore"
)

var LoggerManager *Manager
var DefaultLogger *Logger

func (l *Manager) defaultLogger() *Logger {
	return l.defaultLog
}

func ChooseLogger(name string) *Logger {
	return LoggerManager.logs[name]
}

type Manager struct {
	defaultLog *Logger
	logs       map[string]*Logger
}

// 新建一个logger
func InitLogger(appName string, configs []*LogConfig, defaultLoggerLevel zapcore.Level, args ...interface{}) {

	//初始化一个DefaultLogger
	defaultConfig := NewDefaultConfig(defaultLoggerLevel)

	manager := &Manager{}

	defaultLog := newLog(appName, defaultConfig, args...)
	DefaultLogger = defaultLog

	manager.defaultLog = defaultLog
	manager.logs = map[string]*Logger{}

	for _, config := range configs {
		logger := newLog(appName, config, args...)
		manager.logs[config.Name] = logger
	}
	LoggerManager = manager
}

func Infof(ctx context.Context, template string, args ...interface{}) {
	GetLogger(ctx).Sugar().Infof(template, args...)
}

func Errorf(ctx context.Context, template string, args ...interface{}) {
	GetLogger(ctx).Sugar().Errorf(template, args...)
}

func Warnf(ctx context.Context, template string, args ...interface{}) {
	GetLogger(ctx).Warnf(template, args...)
}

func Debugf(ctx context.Context, template string, args ...interface{}) {
	GetLogger(ctx).Debugf(template, args...)
}

// GetLogger retrieves the current logger from the context. If no logger is
// available, the default logger is returned.
func GetLogger(ctx context.Context) *Logger {
	lc := ctx.Value(LogCtxKey)
	if lc != nil {
		if logCtx, ok := lc.(*loggerContext); ok {
			return logCtx.Logger
		}
	}
	return DefaultLogger
}
