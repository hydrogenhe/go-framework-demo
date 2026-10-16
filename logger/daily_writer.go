package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type dailyWriter struct {
	mu   sync.Mutex
	dir  string
	file *os.File
	date string
}

func newDailyWriter(dir string) (*dailyWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	return &dailyWriter{dir: dir}, nil
}

func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if w.file == nil || w.date != today {
		if w.file != nil {
			_ = w.file.Close()
		}
		file, err := os.OpenFile(filepath.Join(w.dir, "app-"+today+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return 0, err
		}
		w.file, w.date = file, today
	}
	return w.file.Write(p)
}

func (w *dailyWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Sync()
}
