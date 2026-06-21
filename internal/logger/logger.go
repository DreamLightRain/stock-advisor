package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	}
	return "UNKNOWN"
}

type Logger struct {
	mu       sync.Mutex
	baseDir  string
}

func NewLogger(baseDir string) *Logger {
	return &Logger{baseDir: baseDir}
}

func (l *Logger) logDir(date string) string {
	return filepath.Join(l.baseDir, date)
}

func (l *Logger) logFile(date, module string) string {
	return filepath.Join(l.logDir(date), module+".log")
}

func (l *Logger) write(level Level, module, format string, args ...interface{}) {
	now := time.Now()
	date := now.Format("2006-01-02")
	timestamp := now.Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] [%s] [%s] %s\n", timestamp, level.String(), module, msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	dir := l.logDir(date)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	fp := l.logFile(date, module)
	f, err := os.OpenFile(fp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line)
}

func (l *Logger) Debug(module, format string, args ...interface{}) {
	l.write(DEBUG, module, format, args...)
}

func (l *Logger) Info(module, format string, args ...interface{}) {
	l.write(INFO, module, format, args...)
}

func (l *Logger) Warn(module, format string, args ...interface{}) {
	l.write(WARN, module, format, args...)
}

func (l *Logger) Error(module, format string, args ...interface{}) {
	l.write(ERROR, module, format, args...)
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level    string `json:"level"`
	Module   string `json:"module"`
	Message  string `json:"message"`
}

func (l *Logger) GetLogs(dirDate, module string) ([]LogEntry, error) {
	fp := l.logFile(dirDate, module)
	data, err := os.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogEntry{}, nil
		}
		return nil, err
	}
	lines := parseLogLines(string(data))
	return lines, nil
}

func (l *Logger) GetModules(dirDate string) ([]string, error) {
	dir := l.logDir(dirDate)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var modules []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			modules = append(modules, e.Name()[:len(e.Name())-4])
		}
	}
	return modules, nil
}

func (l *Logger) GetDateDirs() ([]string, error) {
	entries, err := os.ReadDir(l.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs, nil
}

func parseLogLines(text string) []LogEntry {
	var entries []LogEntry
	for _, raw := range splitLines(text) {
		// Format: [timestamp] [LEVEL] [module] message
		if len(raw) < 5 || raw[0] != '[' {
			continue
		}
		// Find 4 bracketed sections
		var parts []string
		depth := 0
		start := 0
		for i, ch := range raw {
			if ch == '[' {
				if depth == 0 {
					start = i + 1
				}
				depth++
			} else if ch == ']' {
				depth--
				if depth == 0 {
					parts = append(parts, raw[start:i])
				}
			}
		}
		if len(parts) >= 3 {
			msgStart := 0
			closeCount := 0
			for i, ch := range raw {
				if ch == ']' {
					closeCount++
					if closeCount == 3 {
						msgStart = i + 2
						break
					}
				}
			}
			msg := ""
			if msgStart > 0 && msgStart < len(raw) {
				msg = raw[msgStart:]
			}
			entries = append(entries, LogEntry{
				Timestamp: parts[0],
				Level:    parts[1],
				Module:   parts[2],
				Message:  msg,
			})
		}
	}
	return entries
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				lines = append(lines, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// Map of module names
const (
	ModuleSystem  = "system"
	ModuleData    = "data"
	ModuleAI      = "ai"
	ModuleStorage = "storage"
	ModuleAnalysis = "analysis"
)
