package logging

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const MaxFileSize int64 = 5 * 1024 * 1024

type Config struct {
	Directory string
	Filename  string
	Level     string
	Now       func() time.Time
	MaxBytes  int64
}

type Handle struct {
	handler *rotatingHandler
}

func (h *Handle) Close() error {
	if h == nil || h.handler == nil {
		return nil
	}
	return h.handler.close()
}

func New(config Config) (*slog.Logger, *Handle, error) {
	if config.Directory == "" {
		config.Directory = "logs"
	}
	if config.Filename == "" {
		config.Filename = "sadp-handler.log"
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.MaxBytes == 0 {
		config.MaxBytes = MaxFileSize
	}
	if config.MaxBytes < 1 {
		return nil, nil, errors.New("logging maximum size must be positive")
	}
	level, err := ParseLevel(config.Level)
	if err != nil {
		return nil, nil, err
	}
	if filepath.Base(config.Filename) != config.Filename {
		return nil, nil, errors.New("logging filename must not contain a path")
	}
	if err := os.MkdirAll(config.Directory, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(config.Directory, config.Filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	handler := &rotatingHandler{
		file:      file,
		directory: config.Directory,
		filename:  config.Filename,
		maxBytes:  config.MaxBytes,
		level:     level,
		now:       config.Now,
	}
	if err := handler.writeHeader(); err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("write logging header: %w", err)
	}
	return slog.New(handler), &Handle{handler: handler}, nil
}

func ParseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(value) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid logging level")
	}
}

func SafeAttr(key string, value any) slog.Attr {
	if isSensitiveKey(key) {
		return slog.String(key, "[REDACTED]")
	}
	return slog.Any(key, value)
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(key)
	for _, part := range []string{"password", "secret", "challenge", "token", "payload", "xml", "json", "hash"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

type rotatingHandler struct {
	mu        sync.Mutex
	file      *os.File
	directory string
	filename  string
	maxBytes  int64
	level     slog.Level
	now       func() time.Time
}

func (h *rotatingHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *rotatingHandler) Handle(_ context.Context, record slog.Record) error {
	var buffer bytes.Buffer
	formatter := slog.NewTextHandler(&buffer, &slog.HandlerOptions{Level: h.level, ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		return attr
	}})
	if err := formatter.Handle(context.Background(), record); err != nil {
		return err
	}
	data := buffer.Bytes()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.file == nil {
		return errors.New("logging file is closed")
	}
	if info, err := h.file.Stat(); err != nil {
		return fmt.Errorf("stat log file: %w", err)
	} else if info.Size()+int64(len(data)) > h.maxBytes {
		if err := h.rotate(); err != nil {
			return err
		}
	}
	_, err := h.file.Write(data)
	if err != nil {
		return fmt.Errorf("write log record: %w", err)
	}
	return nil
}

func (h *rotatingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &attrHandler{parent: h, attrs: append([]slog.Attr(nil), attrs...)}
}

func (h *rotatingHandler) WithGroup(name string) slog.Handler {
	return &groupHandler{parent: h, name: name}
}

func (h *rotatingHandler) writeHeader() error {
	header := []byte("\n===== logging started: " + h.now().Format("2006-01-02 15:04:05 -07:00") + " =====\n")
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.file.Write(header)
	return err
}

func (h *rotatingHandler) rotate() error {
	if err := h.file.Close(); err != nil {
		return fmt.Errorf("close log file for rotation: %w", err)
	}
	for index := 1; ; index++ {
		name := fmt.Sprintf("%s.%04d", h.filename, index)
		path := filepath.Join(h.directory, name)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := os.Rename(filepath.Join(h.directory, h.filename), path); err != nil {
				return fmt.Errorf("rotate log file: %w", err)
			}
			break
		} else if err != nil {
			return fmt.Errorf("check rotated log file: %w", err)
		}
	}
	file, err := os.OpenFile(filepath.Join(h.directory, h.filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("reopen log file: %w", err)
	}
	h.file = file
	return nil
}

func (h *rotatingHandler) close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.file == nil {
		return nil
	}
	err := h.file.Close()
	h.file = nil
	return err
}

type attrHandler struct {
	parent *rotatingHandler
	attrs  []slog.Attr
}

func (h *attrHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.parent.Enabled(ctx, level)
}
func (h *attrHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, attr := range h.attrs {
		record.AddAttrs(attr)
	}
	return h.parent.Handle(ctx, record)
}
func (h *attrHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &attrHandler{parent: h.parent, attrs: append(append([]slog.Attr(nil), h.attrs...), attrs...)}
}
func (h *attrHandler) WithGroup(name string) slog.Handler {
	return &groupHandler{parent: h.parent, name: name, attrs: h.attrs}
}

type groupHandler struct {
	parent *rotatingHandler
	name   string
	attrs  []slog.Attr
}

func (h *groupHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.parent.Enabled(ctx, level)
}
func (h *groupHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, attr := range h.attrs {
		record.AddAttrs(attr)
	}
	return h.parent.Handle(ctx, record)
}
func (h *groupHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &groupHandler{parent: h.parent, name: h.name, attrs: append(append([]slog.Attr(nil), h.attrs...), attrs...)}
}
func (h *groupHandler) WithGroup(name string) slog.Handler {
	return &groupHandler{parent: h.parent, name: h.name + "." + name, attrs: h.attrs}
}

var _ io.Closer = (*Handle)(nil)
