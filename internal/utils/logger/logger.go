package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils"
)

var (
	_logger *Logger
)

type Logger struct {
	LogBasePath       string
	LogFileNamePrefix string
	DebugLogger       *log.Logger
	InfoLogger        *log.Logger
	WarnLogger        *log.Logger
	ErrorLogger       *log.Logger
}

func SetupLoggers(logName string, logDir string) {

	_logger = &Logger{}
	_logger.LogBasePath = logDir
	_logger.LogFileNamePrefix = logName

	if err := utils.CreateFolder(_logger.LogBasePath); err != nil {
		log.Fatal(err)
	}

	logFile := filepath.Join(_logger.LogBasePath, _logger.LogFileNamePrefix+"-combined.log")
	errorlogFile := filepath.Join(_logger.LogBasePath, _logger.LogFileNamePrefix+"-error.log")

	if utils.CheckFileExists(logFile) {
		os.Remove(logFile)
	}
	if utils.CheckFileExists(errorlogFile) {
		os.Remove(errorlogFile)
	}

	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	errorf, err := os.OpenFile(errorlogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	wrt := io.MultiWriter(os.Stdout, f)
	errorwrt := io.MultiWriter(wrt, errorf)

	log.SetOutput(wrt)

	_logger.DebugLogger = log.New(os.Stdout, "[ DEBUG ] ", log.Ldate|log.Ltime)
	_logger.InfoLogger = log.New(wrt, "[ INFO ] ", log.Ldate|log.Ltime)
	_logger.WarnLogger = log.New(wrt, "[ WARN ] ", log.Ldate|log.Ltime)
	_logger.ErrorLogger = log.New(errorwrt, "[ ERROR ] ", log.Ldate|log.Ltime)

	GetInfoLogger().Printf("Log File Location: %s", logFile)
}

func GetLogger() *Logger {
	return _logger
}

func GetDebugLogger() *log.Logger {
	return GetLogger().DebugLogger
}

func GetInfoLogger() *log.Logger {
	return GetLogger().InfoLogger
}

func GetWarnLogger() *log.Logger {
	return GetLogger().WarnLogger
}

func GetErrorLogger() *log.Logger {
	return GetLogger().ErrorLogger
}
