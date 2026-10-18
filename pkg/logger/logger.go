package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"whatsgo/pkg/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

var globalLogLevel = LogLevelInfo

func ParseLogLevel(level string) LogLevel {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return LogLevelDebug
	case "INFO":
		return LogLevelInfo
	case "WARN", "WARNING":
		return LogLevelWarn
	case "ERROR":
		return LogLevelError
	case "FATAL":
		return LogLevelFatal
	default:
		return LogLevelInfo
	}
}

func SetGlobalLogLevel(level LogLevel) {
	globalLogLevel = level
}

type LoggerManager struct {
	config       *config.Config
	loggers      map[string]*Logger
	statsTracker *StatsTracker
	mu           sync.RWMutex
}

type Logger struct {
	config     *config.Config
	instanceId string
	level      LogLevel
	mu         sync.Mutex
	writer     *lumberjack.Logger
}

type LogEntry struct {
	Timestamp  time.Time       `json:"timestamp"`
	Level      string          `json:"level"`
	InstanceId string          `json:"instance_id"`
	Message    string          `json:"message"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
}

func NewLoggerManager(config *config.Config) *LoggerManager {
	parsedLevel := ParseLogLevel(config.LogLevel)
	SetGlobalLogLevel(parsedLevel)

	// Garante que o diretório base de logs existe
	if err := os.MkdirAll(config.LogDirectory, 0755); err != nil {
		LogError("Falha ao criar diretório base de logs: %v", err)
	}

	var statsTracker *StatsTracker
	if config.LogSummaryIntervalSec > 0 {
		statsTracker = NewStatsTracker(config.LogSummaryIntervalSec)
	}

	return &LoggerManager{
		config:       config,
		loggers:      make(map[string]*Logger),
		statsTracker: statsTracker,
	}
}

func (lm *LoggerManager) GetStatsTracker() *StatsTracker {
	return lm.statsTracker
}

func (lm *LoggerManager) GetLogger(instanceId string) *Logger {
	lm.mu.RLock()
	logger, exists := lm.loggers[instanceId]
	lm.mu.RUnlock()

	if exists {
		return logger
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Verificar novamente após obter o lock de escrita
	if logger, exists = lm.loggers[instanceId]; exists {
		return logger
	}

	// Criar novo logger para a instância
	logger = newLogger(instanceId, lm.config)
	lm.loggers[instanceId] = logger
	return logger
}

func newLogger(instanceId string, config *config.Config) *Logger {
	// Garante que o diretório existe
	logPath := filepath.Join(config.LogDirectory, instanceId)
	os.MkdirAll(logPath, 0755)

	logFile := filepath.Join(logPath, "instance.log")

	writer := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    config.LogMaxSize,
		MaxBackups: config.LogMaxBackups,
		MaxAge:     config.LogMaxAge,
		Compress:   config.LogCompress,
	}

	return &Logger{
		config:     config,
		instanceId: instanceId,
		level:      ParseLogLevel(config.LogLevel),
		writer:     writer,
	}
}

func (l *Logger) LogInfo(format string, args ...interface{}) {
	if l.level > LogLevelInfo {
		return
	}
	l.log("INFO", format, args...)
	LogInfo(format, args...)
}

func (l *Logger) LogError(format string, args ...interface{}) {
	if l.level > LogLevelError {
		return
	}
	l.log("ERROR", format, args...)
	LogError(format, args...)
}

func (l *Logger) LogWarn(format string, args ...interface{}) {
	if l.level > LogLevelWarn {
		return
	}
	l.log("WARN", format, args...)
	LogWarn(format, args...)
}

func (l *Logger) LogDebug(format string, args ...interface{}) {
	if l.level > LogLevelDebug {
		return
	}
	l.log("DEBUG", format, args...)
	LogDebug(format, args...)
}

func (l *Logger) log(level string, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp:  time.Now(),
		Level:      level,
		InstanceId: l.instanceId,
		Message:    fmt.Sprintf(format, args...),
	}

	jsonEntry, err := json.Marshal(entry)
	if err != nil {
		LogError("Failed to marshal log entry: %v", err)
		return
	}

	if _, err := l.writer.Write(append(jsonEntry, '\n')); err != nil {
		LogError("Failed to write log: %v", err)
	}
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.writer.Close()
}

// Global logger functions
func LogInfo(format string, args ...interface{}) {
	if globalLogLevel > LogLevelInfo {
		return
	}
	log.Printf("[INFO] "+format, args...)
}

func LogError(format string, args ...interface{}) {
	if globalLogLevel > LogLevelError {
		return
	}
	log.Printf("[ERROR] "+format, args...)
}

func LogWarn(format string, args ...interface{}) {
	if globalLogLevel > LogLevelWarn {
		return
	}
	log.Printf("[WARN] "+format, args...)
}

func LogDebug(format string, args ...interface{}) {
	if globalLogLevel > LogLevelDebug {
		return
	}
	log.Printf("[DEBUG] "+format, args...)
}

func LogFatal(format string, args ...interface{}) {
	log.Fatalf("[FATAL] "+format, args...)
}

// GetLogs retorna os logs da instância com filtros opcionais
func (l *Logger) GetLogs(startDate, endDate time.Time, level string, limit int) ([]LogEntry, error) {
	return nil, fmt.Errorf("método movido para instance_service")
}

// ─────────────────────────────────────────────────────────────
// StatsTracker — Agregação periódica de mensagens para log limpo
// ─────────────────────────────────────────────────────────────

type InstanceStats struct {
	MessagesReceived int64
	MediaReceived    int64
	WebhooksSent     int64
	WebhooksFailed   int64
}

type StatsTracker struct {
	mu            sync.Mutex
	intervalSec   int
	instanceStats map[string]*InstanceStats
	stopChan      chan struct{}
}

func NewStatsTracker(intervalSec int) *StatsTracker {
	st := &StatsTracker{
		intervalSec:   intervalSec,
		instanceStats: make(map[string]*InstanceStats),
		stopChan:      make(chan struct{}),
	}
	if intervalSec > 0 {
		go st.startTicker()
	}
	return st
}

func (st *StatsTracker) TrackMessage(instanceId string, isMedia bool) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	stat, exists := st.instanceStats[instanceId]
	if !exists {
		stat = &InstanceStats{}
		st.instanceStats[instanceId] = stat
	}
	stat.MessagesReceived++
	if isMedia {
		stat.MediaReceived++
	}
}

func (st *StatsTracker) TrackWebhook(instanceId string, success bool) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	stat, exists := st.instanceStats[instanceId]
	if !exists {
		stat = &InstanceStats{}
		st.instanceStats[instanceId] = stat
	}
	if success {
		stat.WebhooksSent++
	} else {
		stat.WebhooksFailed++
	}
}

func (st *StatsTracker) startTicker() {
	ticker := time.NewTicker(time.Duration(st.intervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			st.emitSummary()
		case <-st.stopChan:
			return
		}
	}
}

func (st *StatsTracker) emitSummary() {
	st.mu.Lock()
	if len(st.instanceStats) == 0 {
		st.mu.Unlock()
		return
	}

	var totalMsgs int64
	var totalMedia int64
	var totalWhSent int64
	var totalWhFailed int64
	var details []string

	for instId, s := range st.instanceStats {
		if s.MessagesReceived > 0 || s.WebhooksSent > 0 || s.WebhooksFailed > 0 {
			totalMsgs += s.MessagesReceived
			totalMedia += s.MediaReceived
			totalWhSent += s.WebhooksSent
			totalWhFailed += s.WebhooksFailed

			msgPart := fmt.Sprintf("%d", s.MessagesReceived)
			if s.MediaReceived > 0 {
				msgPart += fmt.Sprintf(" (%d mídia)", s.MediaReceived)
			}
			details = append(details, fmt.Sprintf("%s: %s", instId, msgPart))
		}
	}

	// Limpa contadores para a próxima janela
	st.instanceStats = make(map[string]*InstanceStats)
	st.mu.Unlock()

	if totalMsgs > 0 || totalWhFailed > 0 {
		summary := fmt.Sprintf("[stats] Últimos %ds: %d msgs", st.intervalSec, totalMsgs)
		if totalMedia > 0 {
			summary += fmt.Sprintf(" (%d mídias)", totalMedia)
		}
		summary += fmt.Sprintf(" em %d instâncias | %s", len(details), strings.Join(details, ", "))
		if totalWhFailed > 0 {
			summary += fmt.Sprintf(" | ⚠️ webhooks falhos: %d", totalWhFailed)
		}
		// Emite diretamente no log no nível INFO (sempre visível se LOG_LEVEL <= INFO)
		LogInfo("%s", summary)
	}
}

func (st *StatsTracker) Stop() {
	if st == nil {
		return
	}
	select {
	case <-st.stopChan:
	default:
		close(st.stopChan)
	}
}
