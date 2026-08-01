package logger_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/logger"
)

// NewStructured builds a JSON *slog.Logger ready for a log aggregator.
func ExampleNewStructured() {
	var buf strings.Builder

	log := logger.NewStructured(
		logger.WithWriter(&buf),
		logger.WithLevel(slog.LevelInfo),
		logger.WithAttrs(slog.String("service", "orders")),
	)

	log.Info("order accepted", slog.Int("items", 3))
	log.Debug("dropped: below the configured level")

	// One NDJSON object per line, with slog's standard time/level/msg keys —
	// the spelling aggregators already understand. The time field moves every
	// run, so only the stable fields are printed.
	var line map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &line); err != nil {
		fmt.Println("decode:", err)
		return
	}
	fmt.Println(line["level"], line["msg"], line["service"], line["items"])
	// Output: INFO order accepted orders 3
}

// WithFields accumulates context on the way down; FromContext reads it back.
func ExampleWithFields() {
	var buf strings.Builder

	// FromContext decorates slog.Default, so that is where the base logger is
	// installed — once, at startup.
	previous := slog.Default()
	slog.SetDefault(logger.NewStructured(logger.WithWriter(&buf)))
	defer slog.SetDefault(previous)

	// Each call copies rather than mutating, so fields added deeper in a call
	// stack never escape upward while everything below inherits them.
	ctx := logger.WithFields(context.Background(), logger.String("request_id", "req-42"))
	ctx = logger.WithFields(ctx, logger.Int("attempt", 2))

	// Code this deep needs no logger parameter and no wiring — just the context
	// it already has.
	logger.FromContext(ctx).Info("charging the card")

	var line map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &line); err != nil {
		fmt.Println("decode:", err)
		return
	}
	fmt.Println(line["msg"], line["request_id"], line["attempt"])
	// Output: charging the card req-42 2
}
