package basictracer

import (
	"strconv"
	"strings"

	opentracing "github.com/opentracing/opentracing-go"
)

type textMapPropagator struct {
	tracer *tracerImpl
}

const (
	prefixTracerState = "ot-tracer-"
	prefixBaggage     = "ot-baggage-"

	tracerStateFieldCount = 3
	fieldNameTraceID      = prefixTracerState + "traceid"
	fieldNameSpanID       = prefixTracerState + "spanid"
	fieldNameSampled      = prefixTracerState + "sampled"
)

func (p *textMapPropagator) Inject(
	spanContext opentracing.SpanContext,
	opaqueCarrier interface{},
) error {
	sc, ok := spanContext.(SpanContext)
	if !ok {
		return opentracing.ErrInvalidSpanContext
	}
	carrier, ok := opaqueCarrier.(opentracing.TextMapWriter)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}
	carrier.Set(fieldNameTraceID, strconv.FormatUint(sc.TraceID, 16))
	carrier.Set(fieldNameSpanID, strconv.FormatUint(sc.SpanID, 16))
	carrier.Set(fieldNameSampled, strconv.FormatBool(sc.Sampled))

	for k, v := range sc.Baggage {
		carrier.Set(prefixBaggage+k, v)
	}
	return nil
}

func (p *textMapPropagator) Extract(
	opaqueCarrier interface{},
) (opentracing.SpanContext, error) {
	carrier, ok := opaqueCarrier.(opentracing.TextMapReader)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}
	requiredFieldCount := 0
	var traceID, spanID uint64
	var sampled bool
	var err error
	decodedBaggage := make(map[string]string)
	err = carrier.ForeachKey(func(k, v string) error {
		switch strings.ToLower(k) {
		case fieldNameTraceID:
			traceID, err = strconv.ParseUint(v, 16, 64)
			if err != nil {
				return opentracing.ErrSpanContextCorrupted
			}
		case fieldNameSpanID:
			spanID, err = strconv.ParseUint(v, 16, 64)
			if err != nil {
				return opentracing.ErrSpanContextCorrupted
			}
		case fieldNameSampled:
			sampled, err = strconv.ParseBool(v)
			if err != nil {
				return opentracing.ErrSpanContextCorrupted
			}
		default:
			lowercaseK := strings.ToLower(k)
			if strings.HasPrefix(lowercaseK, prefixBaggage) {
				decodedBaggage[strings.TrimPrefix(lowercaseK, prefixBaggage)] = v
			}
			// Balance off the requiredFieldCount++ just below...
			requiredFieldCount--
		}
		requiredFieldCount++
		return nil
	})
	if err != nil {
		return nil, err
	}
	if requiredFieldCount < tracerStateFieldCount {
		if requiredFieldCount == 0 {
			return nil, opentracing.ErrSpanContextNotFound
		}
		return nil, opentracing.ErrSpanContextCorrupted
	}

	return SpanContext{
		TraceID: traceID,
		SpanID:  spanID,
		Sampled: sampled,
		Baggage: decodedBaggage,
	}, nil
}
