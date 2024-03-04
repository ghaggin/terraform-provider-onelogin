package onelogin

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

type LogLevel int

const (
	Trace LogLevel = iota
	Debug
	Info
	Warn
	Error
)

func StringToLogLevel(level string) (LogLevel, error) {
	switch level {
	case "trace":
		return Trace, nil
	case "debug":
		return Debug, nil
	case "info":
		return Info, nil
	case "warn":
		return Warn, nil
	case "error":
		return Error, nil
	default:
		return 0, fmt.Errorf("invalid log level: %v", level)
	}
}

type Logger interface {
	Trace(ctx context.Context, msg string, fields ...map[string]interface{})
	Debug(ctx context.Context, msg string, fields ...map[string]interface{})
	Info(ctx context.Context, msg string, fields ...map[string]interface{})
	Warn(ctx context.Context, msg string, fields ...map[string]interface{})
	Error(ctx context.Context, msg string, fields ...map[string]interface{})
}

type noopLogger struct{}

func NewNoopLogger() Logger {
	return &noopLogger{}
}

func (n *noopLogger) Trace(_ context.Context, msg string, fields ...map[string]interface{}) {}
func (n *noopLogger) Debug(_ context.Context, msg string, fields ...map[string]interface{}) {}
func (n *noopLogger) Info(_ context.Context, msg string, fields ...map[string]interface{})  {}
func (n *noopLogger) Warn(_ context.Context, msg string, fields ...map[string]interface{})  {}
func (n *noopLogger) Error(_ context.Context, msg string, fields ...map[string]interface{}) {}

type devLogger struct {
	level LogLevel
	fout  *os.File
	mu    sync.Mutex
}

func NewDevLogger(level LogLevel, fout *os.File) Logger {
	return &devLogger{
		level: level,
		fout:  fout,
		mu:    sync.Mutex{},
	}
}

func (d *devLogger) Trace(_ context.Context, msg string, fields ...map[string]interface{}) {
	if d.level <= Trace {
		d.log(Trace, msg, fields...)
		return
	}
}

func (d *devLogger) Debug(_ context.Context, msg string, fields ...map[string]interface{}) {
	if d.level <= Debug {
		d.log(Debug, msg, fields...)
		return
	}
}
func (d *devLogger) Info(_ context.Context, msg string, fields ...map[string]interface{}) {
	if d.level <= Info {
		d.log(Info, msg, fields...)
		return
	}
}
func (d *devLogger) Warn(_ context.Context, msg string, fields ...map[string]interface{}) {
	if d.level <= Warn {
		d.log(Warn, msg, fields...)
		return
	}
}
func (d *devLogger) Error(_ context.Context, msg string, fields ...map[string]interface{}) {
	// errors are always printed
	d.log(Error, msg, fields...)
}

func (d *devLogger) log(level LogLevel, msg string, fields ...map[string]interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()

	levelStr := logLevelToString(level)
	fieldsMerged := mergFields(fields...)
	fieldKeys := []string{}
	for k := range fieldsMerged {
		fieldKeys = append(fieldKeys, k)
	}
	sort.StringSlice(fieldKeys).Sort()

	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%v   %v   %v", time.Now().UTC().Format(time.RFC3339), levelStr, msg))
	for _, k := range fieldKeys {
		buf.WriteString(fmt.Sprintf("   %v: %v", k, fieldsMerged[k]))
	}
	buf.WriteString("\n")

	_, err := d.fout.Write(buf.Bytes())
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("error writing log: %v\n", err))
	}
}

func logLevelToString(level LogLevel) string {
	switch level {
	case Trace:
		return "TRACE"
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func mergFields(fields ...map[string]interface{}) map[string]interface{} {
	merged := map[string]interface{}{}

	for _, f := range fields {
		for k, v := range f {
			merged[k] = v
		}
	}

	return merged
}
