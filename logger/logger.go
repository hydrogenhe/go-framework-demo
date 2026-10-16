package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"time"
)

var Log = newNop()
var Sugar = Log.Sugar()

type logIDContextKey struct{}

// WithLogID 将请求日志 ID 放入上下文，供后续业务处理和日志记录使用。
func WithLogID(ctx context.Context, logID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, logIDContextKey{}, logID)
}

// LogID 获取上下文中的请求日志 ID。
func LogID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	logID, _ := ctx.Value(logIDContextKey{}).(string)
	return logID
}

// NewLogID 生成随机日志 ID，用于关联同一请求或消息的日志。
func NewLogID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(raw[:])
}

// WithContext 返回携带上下文请求日志 ID 的 Zap 日志实例。
func WithContext(ctx context.Context) *zap.Logger {
	if logID := LogID(ctx); logID != "" {
		return Log.With(zap.String("log_id", logID))
	}
	return Log
}

// Debug 输出调试日志，并自动携带上下文中的 log_id。
func Debug(ctx context.Context, message string, fields ...any) {
	WithContext(ctx).Sugar().Debugw(message, fields...)
}

// Info 输出信息日志，并自动携带上下文中的 log_id。
func Info(ctx context.Context, message string, fields ...any) {
	WithContext(ctx).Sugar().Infow(message, fields...)
}

// Warn 输出警告日志，并自动携带上下文中的 log_id。
func Warn(ctx context.Context, message string, fields ...any) {
	WithContext(ctx).Sugar().Warnw(message, fields...)
}

// Error 输出错误日志，并自动携带上下文中的 log_id。
func Error(ctx context.Context, message string, fields ...any) {
	WithContext(ctx).Sugar().Errorw(message, fields...)
}

func Init() {
	InitWithDirectory(LogDirectory())
}

func InitWithDirectory(directory string) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	daily, err := newDailyWriter(directory)
	if err != nil {
		Log = zap.NewNop()
		return
	}
	encoder := zapcore.NewJSONEncoder(cfg.EncoderConfig)
	core := zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(daily)), zap.InfoLevel)
	Log = zap.New(core)
	Sugar = Log.Sugar()
}

// LogDirectory 返回日志配置指向的绝对目录。
func LogDirectory() string {
	starts := make([]string, 0, 2)
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(executable))
	}
	for _, start := range starts {
		for dir := start; ; dir = filepath.Dir(dir) {
			configPath := filepath.Join(dir, "conf", "app.yaml")
			if data, err := os.ReadFile(configPath); err == nil {
				var config struct {
					Logs struct {
						Directory string `yaml:"directory"`
					} `yaml:"logs"`
				}
				if yaml.Unmarshal(data, &config) == nil && config.Logs.Directory != "" {
					if filepath.IsAbs(config.Logs.Directory) {
						return config.Logs.Directory
					}
					return filepath.Join(dir, config.Logs.Directory)
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	return "logs"
}

func newNop() *zap.Logger { return zap.NewNop() }

func Sync() { _ = Log.Sync() }
