// Copyright 2015 Matthew Holt and The Kengine Authors
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

package kenginehttp

import (
	"fmt"
	"net/http"

	"github.com/khulnasoft/kengine/v2"
)

func init() {
	kengine.RegisterModule(Subroute{})
}

// Subroute implements a handler that compiles and executes routes.
// This is useful for a batch of routes that all inherit the same
// matchers, or for multiple routes that should be treated as a
// single route.
//
// You can also use subroutes to handle errors from its handlers.
// First the primary routes will be executed, and if they return an
// error, the errors routes will be executed; in that case, an error
// is only returned to the entry point at the server if there is an
// additional error returned from the errors routes.
type Subroute struct {
	// The primary list of routes to compile and execute.
	Routes RouteList `json:"routes,omitempty"`

	// If the primary routes return an error, error handling
	// can be promoted to this configuration instead.
	Errors *HTTPErrorConfig `json:"errors,omitempty"`
}

// KengineModule returns the Kengine module information.
func (Subroute) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.handlers.subroute",
		New: func() kengine.Module { return new(Subroute) },
	}
}

// Provision sets up subrouting.
func (sr *Subroute) Provision(ctx kengine.Context) error {
	if sr.Routes != nil {
		err := sr.Routes.Provision(ctx)
		if err != nil {
			return fmt.Errorf("setting up subroutes: %v", err)
		}
		if sr.Errors != nil {
			err := sr.Errors.Routes.Provision(ctx)
			if err != nil {
				return fmt.Errorf("setting up error subroutes: %v", err)
			}
		}
	}
	return nil
}

func (sr *Subroute) ServeHTTP(w http.ResponseWriter, r *http.Request, next Handler) error {
	subroute := sr.Routes.Compile(next)
	err := subroute.ServeHTTP(w, r)
	if err != nil && sr.Errors != nil {
		r = sr.Errors.WithError(r, err)
		errRoute := sr.Errors.Routes.Compile(next)
		return errRoute.ServeHTTP(w, r)
	}
	return err
}

// Interface guards
var (
	_ kengine.Provisioner = (*Subroute)(nil)
	_ MiddlewareHandler   = (*Subroute)(nil)
)
