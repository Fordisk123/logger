package logger

import (
	"fmt"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
)

func getWriter(logDir, logFileName string, maxAge, maxLogFileMB, maxLogFileNum int, compress bool) io.Writer {
	return &lumberjack.Logger{
		// 日志名称
		Filename: fmt.Sprintf("%s/%s.log", logDir, logFileName),
		// 日志大小限制，单位MB
		MaxSize: maxLogFileMB,
		// 历史日志文件保留天数
		MaxAge: maxAge,
		// 最大保留历史日志数量
		MaxBackups: maxLogFileNum,
		// 本地时区
		LocalTime: true,
		// 历史日志文件压缩标识
		Compress: compress,
	}
}
