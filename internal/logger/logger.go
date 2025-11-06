package logger

import (
	"io"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var Logger zerolog.Logger

// Init initializes the logger with appropriate settings
func Init(debug bool) {
	zerolog.TimeFieldFormat = time.RFC3339

	// Detect if we're running in a terminal
	isTerminal := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	
	// Allow forcing console output via environment variable for testing
	if os.Getenv("FORCE_COLOR") == "1" || os.Getenv("FORCE_PRETTY") == "1" {
		isTerminal = true
	}

	var output io.Writer = os.Stdout
	
	if isTerminal && !debug {
		// Pretty print for terminal with colors
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
			NoColor:    false,
		}
	} else if isTerminal && debug {
		// Pretty print for terminal in debug mode with more details
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
			NoColor:    false,
		}
	}
	// For non-terminal (e.g., piped output), use JSON format

	// Set global log level
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.DebugLevel
	}

	Logger = zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Logger()

	// Set global logger
	log.Logger = Logger
}
