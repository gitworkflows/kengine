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
	kengine.RegisterModule(Invoke{})
}

// Invoke implements a handler that compiles and executes a
// named route that was defined on the server.
//
// EXPERIMENTAL: Subject to change or removal.
type Invoke struct {
	// Name is the key of the named route to execute
	Name string `json:"name,omitempty"`
}

// KengineModule returns the Kengine module information.
func (Invoke) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.handlers.invoke",
		New: func() kengine.Module { return new(Invoke) },
	}
}

func (invoke *Invoke) ServeHTTP(w http.ResponseWriter, r *http.Request, next Handler) error {
	server := r.Context().Value(ServerCtxKey).(*Server)
	if route, ok := server.NamedRoutes[invoke.Name]; ok {
		return route.Compile(next).ServeHTTP(w, r)
	}
	return fmt.Errorf("invoke: route '%s' not found", invoke.Name)
}

// Interface guards
var (
	_ MiddlewareHandler = (*Invoke)(nil)
)
