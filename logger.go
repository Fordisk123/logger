package logger

import (
	"context"
	"fmt"
	"io"
	"os"
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

const LoggerFormatJson = "json"
const LoggerFormatClassic = "classic"

type LogConfig struct {
	Name          string                 `json:"appName" yaml:"appName"`
	EncoderConfig *zapcore.EncoderConfig `json:"encoderConfig,omitempty" yaml:"encoderConfig,omitempty"`

	//是否输出到文件
	FileLog bool `json:"fileLog,omitempty" yaml:"fileLog,omitempty"`

	//持久化参数
	//持久化文件夹不填默认./logs
	LogDir string `json:"logDir,omitempty" yaml:"basePath,omitempty"`
	//日志名称 不填默认是name
	LogFileName string `json:"logFileName,omitempty" yaml:"fileName,omitempty"`
	//最大保存时间，默认是7天，使用默认请传0
	MaxAge int `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
	// 单个文件最大大小，默认是100MB，使用默认请传0
	MaxLogFileMB int `json:"maxLogFileMB,omitempty" yaml:"maxLogFileMB,omitempty"`
	// 最大文件数量 默认10  使用默认请传0
	MaxLogFileNum int `json:"maxLogFileNum,omitempty" yaml:"maxLogFileNum,omitempty"`
	//是否压缩历史日志 默认不压缩
	LogCompress bool `json:"logCompress,omitempty" yaml:"logCompress,omitempty"`

	//是否跟随RUN_MODE环境变量决定输出类型 PROD为json 其他为classic
	LoggerFormatFollowEnv bool `json:"loggerFormatFollowEnv" yaml:"loggerFormatFollowEnv"`
	//输出格式  json or classic 上述配置文件是false的时候生效
	LoggerFormatType string `json:"loggerFormatType" yaml:"loggerFormatType"`
	//日志等级
	Level zapcore.Level

	//是否打印错误日志栈 默认不打印
	LogErrorStack bool `json:"printErrorStack" yaml:"printErrorStack"`
}

func getDefaultEncoderConfig() *zapcore.EncoderConfig {
	return &zapcore.EncoderConfig{
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

type Logger struct {
	*zap.Logger
	writer io.Writer
}

func NewDefaultConfig(level zapcore.Level) *LogConfig {
	return &LogConfig{
		Name:                  "default",
		EncoderConfig:         getDefaultEncoderConfig(),
		FileLog:               false,
		LogDir:                "",
		LogFileName:           "",
		MaxAge:                0,
		MaxLogFileMB:          0,
		MaxLogFileNum:         0,
		LogCompress:           true,
		LoggerFormatFollowEnv: false,
		LoggerFormatType:      LoggerFormatClassic,
		Level:                 level,
		LogErrorStack:         true,
	}
}

func newLog(appName string, config *LogConfig, args ...interface{}) *Logger {
	if config == nil {
		panic("log config must not be nil")
	}

	if config.Name == "" {
		panic("log name must not be empty")
	}

	if config.EncoderConfig == nil {
		config.EncoderConfig = getDefaultEncoderConfig()
	}

	var writer io.Writer
	if config.FileLog {
		if config.MaxAge == 0 {
			config.MaxAge = 7
		}
		if config.LogDir == "" {
			config.LogDir = "./logs"
		}
		_, err := os.Stat(config.LogDir + "/" + config.LogDir)
		if err != nil {
			if os.IsNotExist(err) {
				err := os.MkdirAll(config.LogDir, os.ModePerm)
				if err != nil {
					panic(fmt.Sprintf("mkdir failed![%s]\n", err.Error()))
				}
			}
		}

		if config.MaxLogFileMB == 0 {
			config.MaxLogFileMB = 50
		}
		if config.MaxLogFileNum == 0 {
			config.MaxLogFileNum = 10
		}
		if config.LogFileName == "" {
			config.LogFileName = config.Name
		}
		writer = getWriter(config.LogDir, config.LogFileName, config.MaxAge, config.MaxLogFileMB, config.MaxLogFileNum, config.LogCompress)
	} else {
		writer = os.Stdout
	}

	if config.LoggerFormatFollowEnv {
		switch os.Getenv(RunModeEnvName) {
		case Prod:
			config.LoggerFormatType = LoggerFormatJson
		default:
			config.LoggerFormatType = LoggerFormatClassic
		}
	}
	if config.LoggerFormatType == "" {
		config.LoggerFormatType = LoggerFormatClassic
	}

	levelFunc := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		if config.Level > lvl {
			return false
		} else {
			return true
		}
	})

	var zapCore zapcore.Core
	if config.LoggerFormatType == LoggerFormatJson {
		zapCore = zapcore.NewCore(zapcore.NewJSONEncoder(*config.EncoderConfig), zapcore.AddSync(writer), levelFunc)
	} else {
		zapCore = zapcore.NewCore(zapcore.NewConsoleEncoder(*config.EncoderConfig), zapcore.AddSync(writer), levelFunc)
	}

	var logger *zap.Logger
	if config.LogErrorStack {
		logger = zap.New(zapCore, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	} else {
		logger = zap.New(zapCore, zap.AddCaller(), zap.AddStacktrace(zap.FatalLevel))
	}
	logger.Named(appName)
	logger = logger.With(handleFields(logger, args)...)

	return &Logger{
		Logger: logger,
		writer: writer,
	}
}

// 给logger添加键值对标记，传参必须是偶数个， 2个为一个键值对
func WithFields(ctx context.Context, args ...interface{}) *Logger {
	return withFields(ctx, args...)
}

func withFields(ctx context.Context, args ...interface{}) *Logger {
	if logCtx, ok := getLoggerCtx(ctx); ok {
		ctx = logCtx
		return logCtx.Logger.withFields(logCtx, args...)
	} else {
		if DefaultLogger == nil {
			panic("must init logger first")
		}
		log := &Logger{
			Logger: DefaultLogger.With(handleFields(DefaultLogger.Logger, args)...),
		}
		ctx = WithContext(ctx, log)
		return log
	}
}

func WithContext(ctx context.Context, logger *Logger) context.Context {
	return &loggerContext{
		Context: ctx,
		Logger:  logger,
	}
}

func (log *Logger) GetWriter() io.Writer {
	return log.writer
}

func (log *Logger) JustWithFields(ctx context.Context, args ...interface{}) *Logger {
	return log.withFieldsPure(ctx, args...)
}

// 给logger添加键值对标记，传参必须是偶数个， 2个为一个键值对
func (log *Logger) WithFields(ctx context.Context, args ...interface{}) *Logger {
	return log.withFields(ctx, args...)
}

func (log *Logger) WithMap(ctx context.Context, kvMap map[string]interface{}) *Logger {
	args := make([]interface{}, 0)
	for k, v := range kvMap {
		args = append(args, k)
		args = append(args, v)
	}
	return log.withFields(ctx, args...)
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

func (log *Logger) withFields(ctx context.Context, args ...interface{}) *Logger {
	return log.withFieldsPure(ctx, args...)
}

func (log *Logger) withFieldsPure(ctx context.Context, args ...interface{}) *Logger {
	l := &Logger{
		Logger: log.With(handleFields(log.Logger, args)...),
	}

	lc, ok := getLoggerCtx(ctx)
	if ok {
		lc.Logger = l
	}

	return l
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
