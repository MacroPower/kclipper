package tracing

import (
	"context"
	"log/slog"
	"time"
)

var (
	_ Tracer = LoggingTracer{}
	_ Span   = loggingSpan{}
)

type LoggingTracer struct {
	logger *slog.Logger
}

func NewLoggingTracer(logger *slog.Logger) *LoggingTracer {
	return &LoggingTracer{
		logger: logger,
	}
}

//nolint:ireturn
func (l LoggingTracer) StartSpan(operationName string) Span {
	return loggingSpan{
		logger:        l.logger,
		operationName: operationName,
		baggage:       make(map[string]interface{}),
		start:         time.Now(),
	}
}

type loggingSpan struct {
	logger        *slog.Logger
	operationName string
	baggage       map[string]interface{}
	start         time.Time
}

func (s loggingSpan) Finish() {
	attrs := []any{}
	attrs = append(attrs, baggageToVals(s.baggage)...)
	attrs = append(attrs, "operation_name", s.operationName, "time_ms", time.Since(s.start).Seconds()*1e3)
	s.logger.Log(context.Background(), slog.LevelDebug, "trace", attrs...)
}

func (s loggingSpan) SetBaggageItem(key string, value interface{}) {
	s.baggage[key] = value
}

func baggageToVals(baggage map[string]interface{}) []interface{} {
	result := make([]interface{}, 0, len(baggage)*2)
	for k, v := range baggage {
		result = append(result, k, v)
	}
	return result
}
