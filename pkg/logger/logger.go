package logger

import (
	"log"
	"os"
	"sync"

	"github.com/whywaita/myshoes/internal/config"
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
	if isDebug && !config.Config.Debug {
		return
	}

	logMu.Lock()
	logger.Printf(format, v...)
	logMu.Unlock()
}
