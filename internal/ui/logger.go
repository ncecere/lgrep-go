// Package ui provides terminal UI components and styling for lgrep.
package ui

import (
	"os"

	"github.com/charmbracelet/log"
)

// InitLogger initializes the charm logger with default settings.
func InitLogger() {
	log.SetOutput(os.Stderr)
	log.SetLevel(log.InfoLevel)
	log.SetReportCaller(false)
	log.SetReportTimestamp(false)
}

// SetDebug enables debug logging.
func SetDebug(enabled bool) {
	if enabled {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}
