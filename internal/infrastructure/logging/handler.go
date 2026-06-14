package logging

import (
	"context"
	"log/slog"
	"os"
)

// SplitHandler はログレベルに応じて標準出力と標準エラー出力を振り分ける slog.Handler です。
type SplitHandler struct {
	stdoutHandler slog.Handler
	stderrHandler slog.Handler
}

// NewSplitHandler は SplitHandler のコンストラクタです。
func NewSplitHandler() slog.Handler {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	return &SplitHandler{
		stdoutHandler: slog.NewJSONHandler(os.Stdout, opts),
		stderrHandler: slog.NewJSONHandler(os.Stderr, opts),
	}
}

func (h *SplitHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h *SplitHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError {
		return h.stderrHandler.Handle(ctx, r)
	}
	return h.stdoutHandler.Handle(ctx, r)
}

func (h *SplitHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SplitHandler{
		stdoutHandler: h.stdoutHandler.WithAttrs(attrs),
		stderrHandler: h.stderrHandler.WithAttrs(attrs),
	}
}

func (h *SplitHandler) WithGroup(name string) slog.Handler {
	return &SplitHandler{
		stdoutHandler: h.stdoutHandler.WithGroup(name),
		stderrHandler: h.stderrHandler.WithGroup(name),
	}
}
