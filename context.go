package logger

import (
	"context"
)

const LogCtxKey = "fordisk-log"

type loggerContext struct {
	context.Context
	Logger *Logger
}

func (l *loggerContext) Value(i interface{}) interface{} {
	if key, ok := i.(string); ok && key == LogCtxKey {
		return l
	}
	return l.Context.Value(i)
}
