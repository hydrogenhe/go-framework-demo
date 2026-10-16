package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	appLogger "go-framework-demo/logger"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// SQL 日志开关：当前 Demo 调试阶段记录每条 SQL，并展开参数值。
// 线上或需要隐藏数据时，将 logSQLParameters 改成 false，SQL 会保留 ? 占位符。
const (
	// logAllSQLStatements 是否记录全部SQL语句。
	logAllSQLStatements = true
	// logSQLParameters 是否在SQL日志中记录绑定参数。
	logSQLParameters = true
)

type zapGormLogger struct {
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

func newZapGormLogger(level gormlogger.LogLevel) gormlogger.Interface {
	return zapGormLogger{
		level:         level,
		slowThreshold: 200 * time.Millisecond,
	}
}

func (l zapGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	l.level = level
	return l
}

func (l zapGormLogger) Info(ctx context.Context, message string, data ...interface{}) {
	if l.level >= gormlogger.Info {
		appLogger.Info(ctx, fmt.Sprintf(message, data...), "source", "gorm")
	}
}

func (l zapGormLogger) Warn(ctx context.Context, message string, data ...interface{}) {
	if l.level >= gormlogger.Warn {
		appLogger.Warn(ctx, fmt.Sprintf(message, data...), "source", "gorm")
	}
}

func (l zapGormLogger) Error(ctx context.Context, message string, data ...interface{}) {
	if l.level >= gormlogger.Error {
		appLogger.Error(ctx, fmt.Sprintf(message, data...), "source", "gorm")
	}
}

func (l zapGormLogger) Trace(ctx context.Context, started time.Time, query func() (string, int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(started)
	slow := l.slowThreshold > 0 && elapsed >= l.slowThreshold
	if err == nil && !slow && !logAllSQLStatements {
		return
	}

	sql, rowsAffected := query()
	fields := []any{
		"source", "gorm",
		"sql", sql,
		"rows_affected", rowsAffected,
		"elapsed_ms", float64(elapsed) / float64(time.Millisecond),
	}

	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		appLogger.Error(ctx, "GORM SQL执行失败", append(fields, "error", err.Error())...)
	case slow:
		appLogger.Warn(ctx, "GORM慢SQL", fields...)
	case logAllSQLStatements:
		appLogger.Info(ctx, "GORM SQL执行完成", fields...)
	}
}

// ParamsFilter 控制 GORM 是否在 SQL 日志中展开绑定参数。
func (zapGormLogger) ParamsFilter(_ context.Context, sql string, params ...interface{}) (string, []interface{}) {
	if !logSQLParameters {
		return sql, nil
	}
	return sql, params
}
