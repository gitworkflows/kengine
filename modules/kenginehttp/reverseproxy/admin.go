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

package reverseproxy

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/khulnasoft/kengine/v2"
)

func init() {
	kengine.RegisterModule(adminUpstreams{})
}

// adminUpstreams is a module that provides the
// /reverse_proxy/upstreams endpoint for the Kengine admin
// API. This allows for checking the health of configured
// reverse proxy upstreams in the pool.
type adminUpstreams struct{}

// upstreamStatus holds the status of a particular upstream
type upstreamStatus struct {
	Address     string `json:"address"`
	NumRequests int    `json:"num_requests"`
	Fails       int    `json:"fails"`
}

// KengineModule returns the Kengine module information.
func (adminUpstreams) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "admin.api.reverse_proxy",
		New: func() kengine.Module { return new(adminUpstreams) },
	}
}

// Routes returns a route for the /reverse_proxy/upstreams endpoint.
func (al adminUpstreams) Routes() []kengine.AdminRoute {
	return []kengine.AdminRoute{
		{
			Pattern: "/reverse_proxy/upstreams",
			Handler: kengine.AdminHandlerFunc(al.handleUpstreams),
		},
	}
}

// handleUpstreams reports the status of the reverse proxy
// upstream pool.
func (adminUpstreams) handleUpstreams(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return kengine.APIError{
			HTTPStatus: http.StatusMethodNotAllowed,
			Err:        fmt.Errorf("method not allowed"),
		}
	}

	// Prep for a JSON response
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)

	// Collect the results to respond with
	results := []upstreamStatus{}

	// Iterate over the upstream pool (needs to be fast)
	var rangeErr error
	hosts.Range(func(key, val any) bool {
		address, ok := key.(string)
		if !ok {
			rangeErr = kengine.APIError{
				HTTPStatus: http.StatusInternalServerError,
				Err:        fmt.Errorf("could not type assert upstream address"),
			}
			return false
		}

		upstream, ok := val.(*Host)
		if !ok {
			rangeErr = kengine.APIError{
				HTTPStatus: http.StatusInternalServerError,
				Err:        fmt.Errorf("could not type assert upstream struct"),
			}
			return false
		}

		results = append(results, upstreamStatus{
			Address:     address,
			NumRequests: upstream.NumRequests(),
			Fails:       upstream.Fails(),
		})
		return true
	})

	// If an error happened during the range, return it
	if rangeErr != nil {
		return rangeErr
	}

	err := enc.Encode(results)
	if err != nil {
		return kengine.APIError{
			HTTPStatus: http.StatusInternalServerError,
			Err:        err,
		}
	}

	return nil
}
