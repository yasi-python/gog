package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Level string

const (
	Debug Level = "debug"
	Info  Level = "info"
	Warn  Level = "warn"
	Error Level = "error"
)

type Logger struct {
	mu    sync.Mutex
	level Level
}

func New(levelStr string) *Logger {
	lvl := Info
	switch levelStr {
	case "debug":
		lvl = Debug
	case "info":
		lvl = Info
	case "warn":
		lvl = Warn
	case "error":
		lvl = Error
	}
	return &Logger{level: lvl}
}

func (l *Logger) shouldLog(level Level) bool {
	order := map[Level]int{Debug: 10, Info: 20, Warn: 30, Error: 40}
	return order[level] >= order[l.level]
}

func (l *Logger) log(level Level, msg string, fields map[string]any) {
	if !l.shouldLog(level) {
		return
	}
	entry := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"level": level,
		"msg":   msg,
	}
	for k, v := range fields {
		entry[k] = v
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(entry)
}

func (l *Logger) Debug(msg string, fields ...any) { l.log(Debug, msg, kv(fields...)) }
func (l *Logger) Info(msg string, fields ...any)  { l.log(Info, msg, kv(fields...)) }
func (l *Logger) Warn(msg string, fields ...any)  { l.log(Warn, msg, kv(fields...)) }
func (l *Logger) Error(msg string, fields ...any) { l.log(Error, msg, kv(fields...)) }

func kv(fields ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(fields); i += 2 {
		k, ok := fields[i].(string)
		if !ok {
			k = fmt.Sprintf("f%d", i)
		}
		m[k] = fields[i+1]
	}
	return m
}