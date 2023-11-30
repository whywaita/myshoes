package logger

import (
	"log"
	"os"
	"sync"

	"github.com/whywaita/myshoes/pkg/config"
)

var (
	logger = log.New(os.Stderr, "", log.LstdFlags)
	logMu  sync.Mutex
)

// SetLogger set logger in outside of library
func SetLogger(l *log.Logger) {
	if l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
	}
	logMu.Lock()
	logger = l
	logMu.Unlock()
}

// Logf is interface for logger
func Logf(isDebug bool, format string, v ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()

	switch {
	case !isDebug:
		// normal logging
		logger.Printf(format, v...)
	case isDebug && config.Config.Debug:
		// debug logging
		format = "[DEBUG] " + format
		logger.Printf(format, v...)
	}
}
