package logger

import (
	"context"
	"testing"
	"time"
)

func TestCtxLogger(t *testing.T) {
	l := WithFields(nil, "time", time.Now(),
		"string", "s",
		"int", 2,
		"float", 1.3)
	ctx := WithContext(context.Background(), l)
	demo(ctx)
}

func demo(ctx context.Context) {
	WithFields(ctx, "key1", "demo1")
	WithFields(ctx, "key2", "demo2")
	GetLogger(ctx).WithFields(ctx, "key3", "demo3")

	GetLogger(ctx).Sugar().Infof("hello")
	GetLogger(ctx).Error("error")
}
