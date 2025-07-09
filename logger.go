package sls

import (
	io "io"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger = initDefaultSLSLogger()

func initDefaultSLSLogger() log.Logger {
	logFileName := os.Getenv("SLS_GO_SDK_LOG_FILE_NAME")
	isJsonType := os.Getenv("SLS_GO_SDK_IS_JSON_TYPE")
	logMaxSize := os.Getenv("SLS_GO_SDK_LOG_MAX_SIZE")
	logFileBackupCount := os.Getenv("SLS_GO_SDK_LOG_FILE_BACKUP_COUNT")
	allowLogLevel := os.Getenv("SLS_GO_SDK_ALLOW_LOG_LEVEL")
	return GenerateInnerLogger(logFileName, isJsonType, logMaxSize, logFileBackupCount, allowLogLevel)
}

func getLogLevelFilter(allowLogLevel string) level.Option {
	switch allowLogLevel {
	case "debug":
		return level.AllowDebug()
	case "info":
		return level.AllowInfo()
	case "warn":
		return level.AllowWarn()
	case "error":
		return level.AllowError()
	default:
		return level.AllowInfo()
	}
}

func GenerateInnerLogger(logFileName, isJsonType, logMaxSize, logFileBackupCount, allowLogLevel string) log.Logger {
	var writer io.Writer
	if logFileName == "stdout" || logFileName == "" { // for backward compatibility
		writer = log.NewSyncWriter(os.Stdout)
	} else {
		writer = initLogFlusher(logFileBackupCount, logMaxSize, logFileName)
	}

	var logger log.Logger
	if isJsonType == "true" {
		logger = log.NewJSONLogger(writer)
	} else {
		logger = log.NewLogfmtLogger(writer)
	}

	logger = level.NewFilter(logger, getLogLevelFilter(allowLogLevel))
	return log.With(logger, "time", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
}

func initLogFlusher(logFileBackupCount, logMaxSize, logFileName string) *lumberjack.Logger {
	var newLogMaxSize, newLogFileBackupCount int
	if logMaxSize == "0" {
		newLogMaxSize = 10
	}
	if logFileBackupCount == "0" {
		newLogFileBackupCount = 10
	}
	return &lumberjack.Logger{
		Filename:   logFileName,
		MaxSize:    newLogMaxSize,
		MaxBackups: newLogFileBackupCount,
		Compress:   true,
	}
}
