package tracing

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig/httpkenginefile"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func init() {
	kengine.RegisterModule(Tracing{})
	httpkenginefile.RegisterHandlerDirective("tracing", parseKenginefile)
}

// Tracing implements an HTTP handler that adds support for distributed tracing,
// using OpenTelemetry. This module is responsible for the injection and
// propagation of the trace context. Configure this module via environment
// variables (see https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/sdk-environment-variables.md).
// Some values can be overwritten in the configuration file.
type Tracing struct {
	// SpanName is a span name. It should follow the naming guidelines here:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/api.md#span
	SpanName string `json:"span"`

	// otel implements opentelemetry related logic.
	otel openTelemetryWrapper

	logger *zap.Logger
}

// KengineModule returns the Kengine module information.
func (Tracing) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.handlers.tracing",
		New: func() kengine.Module { return new(Tracing) },
	}
}

// Provision implements kengine.Provisioner.
func (ot *Tracing) Provision(ctx kengine.Context) error {
	ot.logger = ctx.Logger()

	var err error
	ot.otel, err = newOpenTelemetryWrapper(ctx, ot.SpanName)

	return err
}

// Cleanup implements kengine.CleanerUpper and closes any idle connections. It
// calls Shutdown method for a trace provider https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/sdk.md#shutdown.
func (ot *Tracing) Cleanup() error {
	if err := ot.otel.cleanup(ot.logger); err != nil {
		return fmt.Errorf("tracerProvider shutdown: %w", err)
	}
	return nil
}

// ServeHTTP implements kenginehttp.MiddlewareHandler.
func (ot *Tracing) ServeHTTP(w http.ResponseWriter, r *http.Request, next kenginehttp.Handler) error {
	return ot.otel.ServeHTTP(w, r, next)
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens. Syntax:
//
//	tracing {
//	    [span <span_name>]
//	}
func (ot *Tracing) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	setParameter := func(d *kenginefile.Dispenser, val *string) error {
		if d.NextArg() {
			*val = d.Val()
		} else {
			return d.ArgErr()
		}
		if d.NextArg() {
			return d.ArgErr()
		}
		return nil
	}

	// paramsMap is a mapping between "string" parameter from the Kenginefile and its destination within the module
	paramsMap := map[string]*string{
		"span": &ot.SpanName,
	}

	d.Next() // consume directive name
	if d.NextArg() {
		return d.ArgErr()
	}

	for d.NextBlock(0) {
		if dst, ok := paramsMap[d.Val()]; ok {
			if err := setParameter(d, dst); err != nil {
				return err
			}
		} else {
			return d.ArgErr()
		}
	}
	return nil
}

func parseKenginefile(h httpkenginefile.Helper) (kenginehttp.MiddlewareHandler, error) {
	var m Tracing
	err := m.UnmarshalKenginefile(h.Dispenser)
	return &m, err
}

// Interface guards
var (
	_ kengine.Provisioner           = (*Tracing)(nil)
	_ kenginehttp.MiddlewareHandler = (*Tracing)(nil)
	_ kenginefile.Unmarshaler       = (*Tracing)(nil)
)
