package logger

import (
	"context"
	"fmt"
	rotatelogs "github.com/lestrrat/go-file-rotatelogs"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	RunModeEnvName = "RUN_MODE"
)

type RunMode string

const (
	Dev  = "DEV"
	Prod = "PROD"
)

// AppName 是logger文件名前缀
// LogDir 日志存储路径,默认是/var/log，使用默认请传空字符串
// EncoderConfig 自定义的log输出配置，使用默认请传nil
// MaxAge 日志保存时间，默认是30，使用默认请传0
// RotationTime 日志分割时间，默认是1天，使用默认请传0
type Config struct {
	AppName       string                 `json:"appName" yaml:"appName"`
	EncoderConfig *zapcore.EncoderConfig `json:"encoderConfig,omitempty" yaml:"encoderConfig,omitempty"`

	//key携带详细信息
	AddFuncInfoWithKey bool `json:"addFuncInfoWithKey,omitempty" yaml:"addFuncInfoWithKey,omitempty"`

	//持久化参数
	LogDir       string        `json:"logDir,omitempty" yaml:"basePath,omitempty"`
	MaxAge       time.Duration `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
	RotationTime time.Duration `json:"rotationTime,omitempty" yaml:"rotationTime,omitempty"`
}

type Logger struct {
	*zap.Logger
	config *Config
}

var initDefaultLoggerOnce sync.Once
var DefaultLogger *Logger

// 新建一个logger
func NewLogger(config *Config, args ...interface{}) *Logger {
	if config == nil {
		config = &Config{}
	}

	if config.MaxAge == 0 {
		config.MaxAge = time.Hour * 24 * 30
	}

	if config.RotationTime == 0 {
		config.RotationTime = time.Hour * 24
	}

	if config.AppName == "" {
		appNamePaths := strings.Split(os.Args[0], "/")
		config.AppName = appNamePaths[len(appNamePaths)-1]
	}

	if config.EncoderConfig == nil {
		config.EncoderConfig = &zapcore.EncoderConfig{
			MessageKey:    "msg",
			LevelKey:      "level",
			TimeKey:       "time",
			CallerKey:     "caller",
			StacktraceKey: "trace",
			LineEnding:    zapcore.DefaultLineEnding,
			EncodeLevel:   zapcore.CapitalLevelEncoder, //log等级大写 DEBUG ,INFO 等。。
			EncodeCaller:  zapcore.ShortCallerEncoder,  //简短调用栈
			EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
				enc.AppendString(t.Format("2006-01-02 15:04:05"))
			}, //使用可读时间输出
			EncodeDuration: func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
				enc.AppendInt64(int64(d) / 1000000)
			},
		}
	}

	//全等级输出
	allLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return true
	})

	var zapCore zapcore.Core
	switch os.Getenv(RunModeEnvName) {
	case Prod:
		{
			//生产环境中，向控制台输出Json日志
			zapCore = zapcore.NewTee(
				zapcore.NewCore(zapcore.NewJSONEncoder(*config.EncoderConfig), zapcore.AddSync(os.Stdout), allLevel),
			)
		}
	default:
		{
			//开发环境中，向控制台输出经典日志
			zapCore = zapcore.NewTee(
				zapcore.NewCore(zapcore.NewConsoleEncoder(*config.EncoderConfig), zapcore.AddSync(os.Stdout), allLevel),
			)
		}
	}

	logger := zap.New(zapCore, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	logger.Named(config.AppName)
	logger = logger.With(handleFields(logger, args)...)
	DefaultLogger = &Logger{logger, config}
	return DefaultLogger
}

// 给logger添加键值对标记，传参必须是偶数个， 2个为一个键值对
func WithFields(ctx context.Context, args ...interface{}) *Logger {
	return withFields(ctx, 0, args...)
}

func WithMap(ctx context.Context, kvMap map[string]interface{}) *Logger {
	args := make([]interface{}, 0)
	for k, v := range kvMap {
		args = append(args, k)
		args = append(args, v)
	}
	return withFields(ctx, 0, args...)
}

func withFields(ctx context.Context, skip int, args ...interface{}) *Logger {
	if logCtx, ok := getLoggerCtx(ctx); ok {
		return logCtx.Logger.withFields(logCtx, 1, args...)
	} else {
		if DefaultLogger == nil {
			initDefaultLoggerOnce.Do(func() {
				DefaultConfig := &Config{}
				DefaultLogger = NewLogger(DefaultConfig)
			})
		}
		if DefaultLogger.config.AddFuncInfoWithKey {
			args = addFuncInfoWithKey(args, skip+1)
		}
		return &Logger{
			Logger: DefaultLogger.With(handleFields(DefaultLogger.Logger, args)...),
			config: DefaultLogger.config,
		}
	}
}

func WithContext(ctx context.Context, logger *Logger) context.Context {
	return &loggerContext{
		Context: ctx,
		Logger:  logger,
	}
}

func (log *Logger) JustWithFields(ctx context.Context, args ...interface{}) *Logger {
	return log.withFieldsPure(ctx, args...)
}

// 给logger添加键值对标记，传参必须是偶数个， 2个为一个键值对
func (log *Logger) WithFields(ctx context.Context, args ...interface{}) *Logger {
	return log.withFields(ctx, 0, args...)
}

func (log *Logger) WithMap(ctx context.Context, kvMap map[string]interface{}) *Logger {
	args := make([]interface{}, 0)
	for k, v := range kvMap {
		args = append(args, k)
		args = append(args, v)
	}
	return log.withFields(ctx, 0, args...)
}

func (log *Logger) Infof(template string, args ...interface{}) {
	log.Sugar().Infof(template, args...)
}

func (log *Logger) Errorf(template string, args ...interface{}) {
	log.Sugar().Errorf(template, args...)
}

func (log *Logger) Warnf(template string, args ...interface{}) {
	log.Sugar().Warnf(template, args...)
}

func (log *Logger) Debugf(template string, args ...interface{}) {
	log.Sugar().Debugf(template, args...)
}

func (log *Logger) logFields(ctx context.Context, args ...interface{}) *Logger {
	lc, ok := getLoggerCtx(ctx)
	if ok {
		for i := 0; i < len(args); {
			k, v := args[i], args[i+1]
			lc.Logger.Sugar().Info(k, " : ", v)
			i += 2
		}
	}
	return lc.Logger
}

func (log *Logger) withFields(ctx context.Context, skip int, args ...interface{}) *Logger {
	if log.config.AddFuncInfoWithKey {
		args = addFuncInfoWithKey(args, skip+1)
	}
	return log.withFieldsPure(ctx, args...)
}

func (log *Logger) withFieldsPure(ctx context.Context, args ...interface{}) *Logger {
	l := &Logger{
		Logger: log.With(handleFields(log.Logger, args)...),
		config: log.config,
	}

	lc, ok := getLoggerCtx(ctx)
	if ok {
		lc.Logger = l
	}

	return l
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

func getLoggerCtx(ctx context.Context) (*loggerContext, bool) {
	if ctx == nil {
		return nil, false
	}
	lc := ctx.Value(LogCtxKey)
	if lc != nil {
		if logCtx, ok := lc.(*loggerContext); ok {
			return logCtx, true
		}
	}
	return nil, false
}

func getFileWriter(config *Config) io.Writer {
	_, err := os.Stat(config.LogDir + "/" + config.AppName)
	if err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(config.LogDir+"/"+config.AppName, os.ModePerm)
			if err != nil {
				panic(fmt.Sprintf("mkdir failed![%s]\n", err.Error()))
			}
		}
	}

	hook, err := rotatelogs.New(
		config.LogDir+"/"+config.AppName+"/"+"log.%Y-%m-%d",
		rotatelogs.WithLinkName("log"),
		rotatelogs.WithMaxAge(time.Hour*24*7),
		rotatelogs.WithRotationTime(time.Hour*24),
	)
	if err != nil {
		panic(fmt.Sprintf("init rotatelog fail:%s", err.Error()))
	}
	return hook
}

// handleFields converts a bunch of arbitrary key-value pairs into Zap fields.  It takes
// additional pre-converted Zap fields, for use with automatically attached fields, like
// `error`.
func handleFields(l *zap.Logger, args []interface{}, additional ...zap.Field) []zap.Field {
	// a slightly modified version of zap.SugaredLogger.sweetenFields
	if len(args) == 0 {
		// fast-return if we have no suggared fields.
		return additional
	}

	// unlike Zap, we can be pretty sure users aren't passing structured
	// fields (since logr has no concept of that), so guess that we need a
	// little less space.
	fields := make([]zap.Field, 0, len(args)/2+len(additional))
	for i := 0; i < len(args); {
		// check just in case for strongly-typed Zap fields, which is illegal (since
		// it breaks implementation agnosticism), so we can give a better error message.
		if _, ok := args[i].(zap.Field); ok {
			l.DPanic("strongly-typed Zap Field passed to logr", zap.Any("zap field", args[i]))
			break
		}

		// make sure this isn't a mismatched key
		if i == len(args)-1 {
			l.DPanic("odd number of arguments passed as key-value pairs for logging", zap.Any("ignored key", args[i]))
			break
		}

		// process a key-value pair,
		// ensuring that the key is a string
		key, val := args[i], args[i+1]
		keyStr, isString := key.(string)
		if !isString {
			// if the key isn't a string, DPanic and stop logging
			l.DPanic("non-string key argument passed to logging, ignoring all later arguments", zap.Any("invalid key", key))
			break
		}

		fields = append(fields, zap.Any(keyStr, val))
		i += 2
	}

	return append(fields, additional...)
}

func addFuncInfoWithKey(args []interface{}, skip int) []interface{} {
	res := make([]interface{}, 0, len(args))

	if len(args)%2 == 1 {
		panic("args must be even")
	}

	for i := 0; i < len(args); {
		key, value := args[i], args[i+1]
		keyStr, isString := key.(string)
		if !isString {
			// if the key isn't a string, DPanic and stop logging
			panic("args key must be string")
		}
		res = append(res, getFuncAndLine(skip)+" -- "+keyStr, value)
		i += 2
	}

	return res
}

// skip 记录内部调用层级，每有一次内部函数栈调用就需要+1，基础是3
func getFuncAndLine(skip int) string {
	pc, _, line, ok := runtime.Caller(3 + skip)
	if ok {
		return fmt.Sprintf("%s:%d", runtime.FuncForPC(pc).Name(), line)
	} else {
		return ""
	}
}
