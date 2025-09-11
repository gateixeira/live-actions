package logger

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type testEncoder struct {
	bytes.Buffer
}

// Implement all required methods of zapcore.PrimitiveArrayEncoder
func (e *testEncoder) AppendString(s string)                      { e.WriteString(s) }
func (e *testEncoder) AppendBool(bool)                            {}
func (e *testEncoder) AppendByteString([]byte)                    {}
func (e *testEncoder) AppendComplex128(complex128)                {}
func (e *testEncoder) AppendComplex64(complex64)                  {}
func (e *testEncoder) AppendFloat64(float64)                      {}
func (e *testEncoder) AppendFloat32(float32)                      {}
func (e *testEncoder) AppendInt(int)                              {}
func (e *testEncoder) AppendInt64(int64)                          {}
func (e *testEncoder) AppendInt32(int32)                          {}
func (e *testEncoder) AppendInt16(int16)                          {}
func (e *testEncoder) AppendInt8(int8)                            {}
func (e *testEncoder) AppendUint(uint)                            {}
func (e *testEncoder) AppendUint64(uint64)                        {}
func (e *testEncoder) AppendUint32(uint32)                        {}
func (e *testEncoder) AppendUint16(uint16)                        {}
func (e *testEncoder) AppendUint8(uint8)                          {}
func (e *testEncoder) AppendUintptr(uintptr)                      {}
func (e *testEncoder) AppendDuration(time.Duration)               {}
func (e *testEncoder) AppendTime(time.Time)                       {}
func (e *testEncoder) AppendArray(zapcore.ArrayMarshaler) error   { return nil }
func (e *testEncoder) AppendObject(zapcore.ObjectMarshaler) error { return nil }
func (e *testEncoder) AppendReflected(interface{}) error          { return nil }

func TestCustomLevelEncoder(t *testing.T) {
	tests := []struct {
		name     string
		level    zapcore.Level
		expected string
	}{
		{
			name:     "Info Level",
			level:    zapcore.InfoLevel,
			expected: colorBlue + "[INFO]" + colorReset,
		},
		{
			name:     "Warning Level",
			level:    zapcore.WarnLevel,
			expected: colorYellow + "[WARN]" + colorReset,
		},
		{
			name:     "Error Level",
			level:    zapcore.ErrorLevel,
			expected: colorRed + "[ERROR]" + colorReset,
		},
		{
			name:     "Debug Level",
			level:    zapcore.DebugLevel,
			expected: colorGreen + "[DEBUG]" + colorReset,
		},
		{
			name:     "Default Level",
			level:    zapcore.Level(99),
			expected: "[LEVEL(99)]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := &testEncoder{}
			customLevelEncoder(tt.level, enc)
			if enc.String() != tt.expected {
				t.Errorf("customLevelEncoder() = %v, want %v", enc.String(), tt.expected)
			}
		})
	}
}

func TestCustomTimeEncoder(t *testing.T) {
	enc := &testEncoder{}
	testTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	expected := colorBlue + "[2024-01-01 12:00:00]" + colorReset

	customTimeEncoder(testTime, enc)

	if enc.String() != expected {
		t.Errorf("customTimeEncoder() = %v, want %v", enc.String(), expected)
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		name   string
		str    string
		length int
		want   string
	}{
		{
			name:   "String shorter than length",
			str:    "test",
			length: 8,
			want:   "test    ",
		},
		{
			name:   "String equal to length",
			str:    "test",
			length: 4,
			want:   "test",
		},
		{
			name:   "String longer than length",
			str:    "testing",
			length: 4,
			want:   "testing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := padRight(tt.str, tt.length)
			if got != tt.want {
				t.Errorf("padRight() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInitLogger(t *testing.T) {
	InitLogger("debug")
	if Logger == nil {
		t.Error("InitLogger() failed to initialize logger")
	}

	// Test logging at different levels
	Logger.Info("test info message")
	Logger.Debug("test debug message")
	Logger.Warn("test warning message")
	Logger.Error("test error message")
}

func TestCustomCallerEncoder(t *testing.T) {
	InitLogger("debug")
	enc := &testEncoder{}
	caller := zapcore.EntryCaller{Defined: true, File: "test/file.go", Line: 42}

	customCallerEncoder(caller, enc)

	result := enc.String()
	if !strings.Contains(result, "test/file.go") {
		t.Errorf("customCallerEncoder() output %q doesn't contain expected file path", result)
	}
}

func TestLoggerLevels(t *testing.T) {
	InitLogger("debug")

	tests := []struct {
		name  string
		level zapcore.Level
		log   func(string, ...zap.Field)
	}{
		{"Debug", zapcore.DebugLevel, Logger.Debug},
		{"Info", zapcore.InfoLevel, Logger.Info},
		{"Warn", zapcore.WarnLevel, Logger.Warn},
		{"Error", zapcore.ErrorLevel, Logger.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !Logger.Core().Enabled(tt.level) {
				t.Errorf("Logger level %v should be enabled", tt.level)
			}
			tt.log("test message")
		})
	}
}
