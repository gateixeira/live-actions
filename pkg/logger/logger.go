package logger

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

var logLevels = map[string]zapcore.Level{
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
}

func InitLogger(level string) {
	config := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		MessageKey:     "msg",
		CallerKey:      "caller",
		NameKey:        "logger",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    customLevelEncoder,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   customCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}

	l, ok := logLevels[level]
	if !ok {
		l = zapcore.InfoLevel // Default to InfoLevel if invalid level provided
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(config),
		zapcore.AddSync(os.Stdout),
		l,
	)

	// Only add caller for debug level
	Logger = zap.New(core, zap.AddCallerSkip(1))
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
)

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	// Add blue color to timestamp with brackets
	const colorBlue = "\033[34m"
	const colorReset = "\033[0m"
	timeStr := colorBlue + "[" + t.Format("2006-01-02 15:04:05") + "]" + colorReset
	enc.AppendString(timeStr)
}

func SyncLogger() {
	_ = Logger.Sync()
}

func customLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	// Color codes

	// Add colors based on log level
	var levelStr string
	switch level {
	case zapcore.InfoLevel:
		levelStr = colorBlue + "[" + level.CapitalString() + "]" + colorReset
	case zapcore.WarnLevel:
		levelStr = colorYellow + "[" + level.CapitalString() + "]" + colorReset
	case zapcore.ErrorLevel:
		levelStr = colorRed + "[" + level.CapitalString() + "]" + colorReset
	case zapcore.DebugLevel:
		levelStr = colorGreen + "[" + level.CapitalString() + "]" + colorReset
	default:
		levelStr = "[" + level.CapitalString() + "]"
	}

	enc.AppendString(levelStr)
}

func customCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	// Only show caller for debug level logs
	if Logger.Core().Enabled(zapcore.DebugLevel) {
		const colorDim = "\033[2m"
		const colorReset = "\033[0m"
		callerStr := colorDim + padRight(caller.TrimmedPath(), 30) + colorReset
		enc.AppendString(callerStr)
	}
}

// Helper function to pad strings
func padRight(str string, length int) string {
	if len(str) >= length {
		return str
	}
	padding := length - len(str)
	padded := str
	for i := 0; i < padding; i++ {
		padded += " "
	}
	return padded
}
