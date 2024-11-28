package logger

import (
	"context"
	"go.uber.org/zap/zapcore"
	"testing"
)

func TestCtxLogger(t *testing.T) {

	configs := []*LogConfig{
		{
			Name:                  "log1",
			EncoderConfig:         nil,
			FileLog:               true,
			LogDir:                "./testdata/log1",
			LogFileName:           "",
			MaxAge:                0,
			MaxLogFileMB:          0,
			MaxLogFileNum:         0,
			LogCompress:           true,
			LoggerFormatFollowEnv: false,
			LoggerFormatType:      LoggerFormatJson,
			Level:                 zapcore.DebugLevel,
		},
	}

	InitLogger("test", configs, zapcore.DebugLevel, "version", "1.0.0")

	ctx := context.Background()
	WithFields(ctx, "namespace", "kube", "name", "test")

	ctx2 := WithContext(context.Background(), ChooseLogger("log1"))
	Infof(ctx2, "hello")

	demo(ctx)
}

func demo(ctx context.Context) {

	WithFields(ctx, "key1", "demo1")
	WithFields(ctx, "key2", "demo2")
	GetLogger(ctx).WithFields(ctx, "key3", "demo3")

	GetLogger(ctx).Sugar().Infof("hello")
	GetLogger(ctx).Error("error")

}
