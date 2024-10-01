// Copyright 2020 Matthew Holt and The Kengine Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig/httpkenginefile"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func init() {
	kengine.RegisterModule(Metrics{})
	httpkenginefile.RegisterHandlerDirective("metrics", parseKenginefile)
}

// Metrics is a module that serves a /metrics endpoint so that any gathered
// metrics can be exposed for scraping. This module is configurable by end-users
// unlike AdminMetrics.
type Metrics struct {
	metricsHandler http.Handler

	// Disable OpenMetrics negotiation, enabled by default. May be necessary if
	// the produced metrics cannot be parsed by the service scraping metrics.
	DisableOpenMetrics bool `json:"disable_openmetrics,omitempty"`
}

// KengineModule returns the Kengine module information.
func (Metrics) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.handlers.metrics",
		New: func() kengine.Module { return new(Metrics) },
	}
}

type zapLogger struct {
	zl *zap.Logger
}

func (l *zapLogger) Println(v ...any) {
	l.zl.Sugar().Error(v...)
}

// Provision sets up m.
func (m *Metrics) Provision(ctx kengine.Context) error {
	log := ctx.Logger()
	m.metricsHandler = createMetricsHandler(&zapLogger{log}, !m.DisableOpenMetrics)
	return nil
}

func parseKenginefile(h httpkenginefile.Helper) (kenginehttp.MiddlewareHandler, error) {
	var m Metrics
	err := m.UnmarshalKenginefile(h.Dispenser)
	return m, err
}

// UnmarshalKenginefile sets up the handler from Kenginefile tokens. Syntax:
//
//	metrics [<matcher>] {
//	    disable_openmetrics
//	}
func (m *Metrics) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume directive name
	args := d.RemainingArgs()
	if len(args) > 0 {
		return d.ArgErr()
	}

	for d.NextBlock(0) {
		switch d.Val() {
		case "disable_openmetrics":
			m.DisableOpenMetrics = true
		default:
			return d.Errf("unrecognized subdirective %q", d.Val())
		}
	}
	return nil
}

func (m Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request, next kenginehttp.Handler) error {
	m.metricsHandler.ServeHTTP(w, r)
	return nil
}

// Interface guards
var (
	_ kengine.Provisioner           = (*Metrics)(nil)
	_ kenginehttp.MiddlewareHandler = (*Metrics)(nil)
	_ kenginefile.Unmarshaler       = (*Metrics)(nil)
)

func createMetricsHandler(logger promhttp.Logger, enableOpenMetrics bool) http.Handler {
	return promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer,
		promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
			// will only log errors if logger is non-nil
			ErrorLog: logger,

			// Allow OpenMetrics format to be negotiated - largely compatible,
			// except quantile/le label values always have a decimal.
			EnableOpenMetrics: enableOpenMetrics,
		}),
	)
}
