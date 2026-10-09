package bootstrap

import (
	"io"
	"os"
	"path"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func SetUpLogger(logFile string, quietMode bool, logLevel string, consoleOutput io.Writer) (*zap.SugaredLogger, error) {

	// Setup the logger output
	if len(logFile) == 0 {
		//logFile = "karr-" + time.Now().Format("20060102T150405") + ".log"
		logFile = "stfg.log"
	} else {
		basePath := path.Dir(logFile)
		if err := os.MkdirAll(basePath, 0777); err != nil {
			return nil, err
		}
	}
	var f *os.File
	var err error

	if f, err = os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err != nil {
		return nil, err
	}

	// configuration settings
	pecfg := zap.NewProductionEncoderConfig()
	pecfg.EncodeTime = zapcore.ISO8601TimeEncoder

	// Configure output encoders
	fileEncoder := zapcore.NewJSONEncoder(pecfg)
	consoleEncoder := zapcore.NewConsoleEncoder(pecfg)

	// Configure log levels
	atomicLevel, err := zap.ParseAtomicLevel(logLevel)
	if err != nil {
		return nil, err
	}

	// Configure outputs based on params
	var core zapcore.Core
	if quietMode {
		core = zapcore.NewTee(
			zapcore.NewCore(fileEncoder, zapcore.AddSync(f), atomicLevel),
		)
	} else {
		core = zapcore.NewTee(
			zapcore.NewCore(fileEncoder, zapcore.AddSync(f), atomicLevel),
			zapcore.NewCore(consoleEncoder, zapcore.Lock(zapcore.AddSync(consoleOutput)), atomicLevel),
		)
	}

	//Build the logger
	logger := zap.New(core).Sugar()

	return logger, nil
}
