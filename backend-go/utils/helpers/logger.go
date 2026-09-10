package helpers

import "go.uber.org/zap/zapcore"

// IsValidLoggerLevel reports whether s is a level zap can parse.
func IsValidLoggerLevel(s string) bool {
	var l zapcore.Level
	return l.UnmarshalText([]byte(s)) == nil
}
