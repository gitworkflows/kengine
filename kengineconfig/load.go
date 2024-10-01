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

package kengineconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"

	"github.com/khulnasoft/kengine/v2"
)

func init() {
	kengine.RegisterModule(adminLoad{})
}

// adminLoad is a module that provides the /load endpoint
// for the Kengine admin API. The only reason it's not baked
// into the kengine package directly is because of the import
// of the kengineconfig package for its GetAdapter function.
// If the kengine package depends on the kengineconfig package,
// then the kengineconfig package will not be able to import
// the kengine package, and it can more easily cause backward
// edges in the dependency tree (i.e. import cycle).
// Fortunately, the admin API has first-class support for
// adding endpoints from modules.
type adminLoad struct{}

// KengineModule returns the Kengine module information.
func (adminLoad) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "admin.api.load",
		New: func() kengine.Module { return new(adminLoad) },
	}
}

// Routes returns a route for the /load endpoint.
func (al adminLoad) Routes() []kengine.AdminRoute {
	return []kengine.AdminRoute{
		{
			Pattern: "/load",
			Handler: kengine.AdminHandlerFunc(al.handleLoad),
		},
		{
			Pattern: "/adapt",
			Handler: kengine.AdminHandlerFunc(al.handleAdapt),
		},
	}
}

// handleLoad replaces the entire current configuration with
// a new one provided in the response body. It supports config
// adapters through the use of the Content-Type header. A
// config that is identical to the currently-running config
// will be a no-op unless Cache-Control: must-revalidate is set.
func (adminLoad) handleLoad(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return kengine.APIError{
			HTTPStatus: http.StatusMethodNotAllowed,
			Err:        fmt.Errorf("method not allowed"),
		}
	}

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	_, err := io.Copy(buf, r.Body)
	if err != nil {
		return kengine.APIError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("reading request body: %v", err),
		}
	}
	body := buf.Bytes()

	// if the config is formatted other than Kengine's native
	// JSON, we need to adapt it before loading it
	if ctHeader := r.Header.Get("Content-Type"); ctHeader != "" {
		result, warnings, err := adaptByContentType(ctHeader, body)
		if err != nil {
			return kengine.APIError{
				HTTPStatus: http.StatusBadRequest,
				Err:        err,
			}
		}
		if len(warnings) > 0 {
			respBody, err := json.Marshal(warnings)
			if err != nil {
				kengine.Log().Named("admin.api.load").Error(err.Error())
			}
			_, _ = w.Write(respBody)
		}
		body = result
	}

	forceReload := r.Header.Get("Cache-Control") == "must-revalidate"

	err = kengine.Load(body, forceReload)
	if err != nil {
		return kengine.APIError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("loading config: %v", err),
		}
	}

	kengine.Log().Named("admin.api").Info("load complete")

	return nil
}

// handleAdapt adapts the given Kengine config to JSON and responds with the result.
func (adminLoad) handleAdapt(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return kengine.APIError{
			HTTPStatus: http.StatusMethodNotAllowed,
			Err:        fmt.Errorf("method not allowed"),
		}
	}

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	_, err := io.Copy(buf, r.Body)
	if err != nil {
		return kengine.APIError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("reading request body: %v", err),
		}
	}

	result, warnings, err := adaptByContentType(r.Header.Get("Content-Type"), buf.Bytes())
	if err != nil {
		return kengine.APIError{
			HTTPStatus: http.StatusBadRequest,
			Err:        err,
		}
	}

	out := struct {
		Warnings []Warning       `json:"warnings,omitempty"`
		Result   json.RawMessage `json:"result"`
	}{
		Warnings: warnings,
		Result:   result,
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(out)
}

// adaptByContentType adapts body to Kengine JSON using the adapter specified by contentType.
// If contentType is empty or ends with "/json", the input will be returned, as a no-op.
func adaptByContentType(contentType string, body []byte) ([]byte, []Warning, error) {
	// assume JSON as the default
	if contentType == "" {
		return body, nil, nil
	}

	ct, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, nil, kengine.APIError{
			HTTPStatus: http.StatusBadRequest,
			Err:        fmt.Errorf("invalid Content-Type: %v", err),
		}
	}

	// if already JSON, no need to adapt
	if strings.HasSuffix(ct, "/json") {
		return body, nil, nil
	}

	// adapter name should be suffix of MIME type
	_, adapterName, slashFound := strings.Cut(ct, "/")
	if !slashFound {
		return nil, nil, fmt.Errorf("malformed Content-Type")
	}

	cfgAdapter := GetAdapter(adapterName)
	if cfgAdapter == nil {
		return nil, nil, fmt.Errorf("unrecognized config adapter '%s'", adapterName)
	}

	result, warnings, err := cfgAdapter.Adapt(body, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("adapting config using %s adapter: %v", adapterName, err)
	}

	return result, warnings, nil
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}
