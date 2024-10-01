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
	"fmt"
	"net/http"
	"strconv"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func init() {
	kengine.RegisterModule(CopyResponseHandler{})
	kengine.RegisterModule(CopyResponseHeadersHandler{})
}

// CopyResponseHandler is a special HTTP handler which may
// only be used within reverse_proxy's handle_response routes,
// to copy the proxy response. EXPERIMENTAL, subject to change.
type CopyResponseHandler struct {
	// To write the upstream response's body but with a different
	// status code, set this field to the desired status code.
	StatusCode kenginehttp.WeakString `json:"status_code,omitempty"`

	ctx kengine.Context
}

// KengineModule returns the Kengine module information.
func (CopyResponseHandler) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.handlers.copy_response",
		New: func() kengine.Module { return new(CopyResponseHandler) },
	}
}

// Provision ensures that h is set up properly before use.
func (h *CopyResponseHandler) Provision(ctx kengine.Context) error {
	h.ctx = ctx
	return nil
}

// ServeHTTP implements the Handler interface.
func (h CopyResponseHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request, _ kenginehttp.Handler) error {
	repl := req.Context().Value(kengine.ReplacerCtxKey).(*kengine.Replacer)
	hrc, ok := req.Context().Value(proxyHandleResponseContextCtxKey).(*handleResponseContext)

	// don't allow this to be used outside of handle_response routes
	if !ok {
		return kenginehttp.Error(http.StatusInternalServerError,
			fmt.Errorf("cannot use 'copy_response' outside of reverse_proxy's handle_response routes"))
	}

	// allow a custom status code to be written; otherwise the
	// status code from the upstream response is written
	if codeStr := h.StatusCode.String(); codeStr != "" {
		intVal, err := strconv.Atoi(repl.ReplaceAll(codeStr, ""))
		if err != nil {
			return kenginehttp.Error(http.StatusInternalServerError, err)
		}
		hrc.response.StatusCode = intVal
	}

	// make sure the reverse_proxy handler doesn't try to call
	// finalizeResponse again after we've already done it here.
	hrc.isFinalized = true

	// write the response
	return hrc.handler.finalizeResponse(rw, req, hrc.response, repl, hrc.start, hrc.logger)
}

// CopyResponseHeadersHandler is a special HTTP handler which may
// only be used within reverse_proxy's handle_response routes,
// to copy headers from the proxy response. EXPERIMENTAL;
// subject to change.
type CopyResponseHeadersHandler struct {
	// A list of header fields to copy from the response.
	// Cannot be defined at the same time as Exclude.
	Include []string `json:"include,omitempty"`

	// A list of header fields to skip copying from the response.
	// Cannot be defined at the same time as Include.
	Exclude []string `json:"exclude,omitempty"`

	includeMap map[string]struct{}
	excludeMap map[string]struct{}
	ctx        kengine.Context
}

// KengineModule returns the Kengine module information.
func (CopyResponseHeadersHandler) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.handlers.copy_response_headers",
		New: func() kengine.Module { return new(CopyResponseHeadersHandler) },
	}
}

// Validate ensures the h's configuration is valid.
func (h *CopyResponseHeadersHandler) Validate() error {
	if len(h.Exclude) > 0 && len(h.Include) > 0 {
		return fmt.Errorf("cannot define both 'exclude' and 'include' lists at the same time")
	}

	return nil
}

// Provision ensures that h is set up properly before use.
func (h *CopyResponseHeadersHandler) Provision(ctx kengine.Context) error {
	h.ctx = ctx

	// Optimize the include list by converting it to a map
	if len(h.Include) > 0 {
		h.includeMap = map[string]struct{}{}
	}
	for _, field := range h.Include {
		h.includeMap[http.CanonicalHeaderKey(field)] = struct{}{}
	}

	// Optimize the exclude list by converting it to a map
	if len(h.Exclude) > 0 {
		h.excludeMap = map[string]struct{}{}
	}
	for _, field := range h.Exclude {
		h.excludeMap[http.CanonicalHeaderKey(field)] = struct{}{}
	}

	return nil
}

// ServeHTTP implements the Handler interface.
func (h CopyResponseHeadersHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request, next kenginehttp.Handler) error {
	hrc, ok := req.Context().Value(proxyHandleResponseContextCtxKey).(*handleResponseContext)

	// don't allow this to be used outside of handle_response routes
	if !ok {
		return kenginehttp.Error(http.StatusInternalServerError,
			fmt.Errorf("cannot use 'copy_response_headers' outside of reverse_proxy's handle_response routes"))
	}

	for field, values := range hrc.response.Header {
		// Check the include list first, skip
		// the header if it's _not_ in this list.
		if len(h.includeMap) > 0 {
			if _, ok := h.includeMap[field]; !ok {
				continue
			}
		}

		// Then, check the exclude list, skip
		// the header if it _is_ in this list.
		if len(h.excludeMap) > 0 {
			if _, ok := h.excludeMap[field]; ok {
				continue
			}
		}

		// Copy all the values for the header.
		for _, value := range values {
			rw.Header().Add(field, value)
		}
	}

	return next.ServeHTTP(rw, req)
}

// Interface guards
var (
	_ kenginehttp.MiddlewareHandler = (*CopyResponseHandler)(nil)
	_ kenginefile.Unmarshaler       = (*CopyResponseHandler)(nil)
	_ kengine.Provisioner           = (*CopyResponseHandler)(nil)

	_ kenginehttp.MiddlewareHandler = (*CopyResponseHeadersHandler)(nil)
	_ kenginefile.Unmarshaler       = (*CopyResponseHeadersHandler)(nil)
	_ kengine.Provisioner           = (*CopyResponseHeadersHandler)(nil)
	_ kengine.Validator             = (*CopyResponseHeadersHandler)(nil)
)
