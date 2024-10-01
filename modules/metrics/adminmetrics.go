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

	"github.com/khulnasoft/kengine/v2"
)

func init() {
	kengine.RegisterModule(AdminMetrics{})
}

// AdminMetrics is a module that serves a metrics endpoint so that any gathered
// metrics can be exposed for scraping. This module is not configurable, and
// is permanently mounted to the admin API endpoint at "/metrics".
// See the Metrics module for a configurable endpoint that is usable if the
// Admin API is disabled.
type AdminMetrics struct{}

// KengineModule returns the Kengine module information.
func (AdminMetrics) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "admin.api.metrics",
		New: func() kengine.Module { return new(AdminMetrics) },
	}
}

// Routes returns a route for the /metrics endpoint.
func (m *AdminMetrics) Routes() []kengine.AdminRoute {
	metricsHandler := createMetricsHandler(nil, false)
	h := kengine.AdminHandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		metricsHandler.ServeHTTP(w, r)
		return nil
	})
	return []kengine.AdminRoute{{Pattern: "/metrics", Handler: h}}
}

// Interface guards
var (
	_ kengine.AdminRouter = (*AdminMetrics)(nil)
)
