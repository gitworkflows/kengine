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

package intercept

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig/httpkenginefile"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func init() {
	kengine.RegisterModule(Intercept{})
	httpkenginefile.RegisterHandlerDirective("intercept", parseKenginefile)
}

// Intercept is a middleware that intercepts then replaces or modifies the original response.
// It can, for instance, be used to implement X-Sendfile/X-Accel-Redirect-like features
// when using modules like FrankenPHP or Kengine Snake.
//
// EXPERIMENTAL: Subject to change or removal.
type Intercept struct {
	// List of handlers and their associated matchers to evaluate
	// after successful response generation.
	// The first handler that matches the original response will
	// be invoked. The original response body will not be
	// written to the client;
	// it is up to the handler to finish handling the response.
	//
	// Three new placeholders are available in this handler chain:
	// - `{http.intercept.status_code}` The status code from the response
	// - `{http.intercept.header.*}` The headers from the response
	HandleResponse []kenginehttp.ResponseHandler `json:"handle_response,omitempty"`

	// Holds the named response matchers from the Kenginefile while adapting
	responseMatchers map[string]kenginehttp.ResponseMatcher

	// Holds the handle_response Kenginefile tokens while adapting
	handleResponseSegments []*kenginefile.Dispenser

	logger *zap.Logger
}

// KengineModule returns the Kengine module information.
//
// EXPERIMENTAL: Subject to change or removal.
func (Intercept) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.handlers.intercept",
		New: func() kengine.Module { return new(Intercept) },
	}
}

// Provision ensures that i is set up properly before use.
//
// EXPERIMENTAL: Subject to change or removal.
func (irh *Intercept) Provision(ctx kengine.Context) error {
	// set up any response routes
	for i, rh := range irh.HandleResponse {
		err := rh.Provision(ctx)
		if err != nil {
			return fmt.Errorf("provisioning response handler %d: %w", i, err)
		}
	}

	irh.logger = ctx.Logger()

	return nil
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// TODO: handle status code replacement
//
// EXPERIMENTAL: Subject to change or removal.
type interceptedResponseHandler struct {
	kenginehttp.ResponseRecorder
	replacer     *kengine.Replacer
	handler      kenginehttp.ResponseHandler
	handlerIndex int
	statusCode   int
}

// EXPERIMENTAL: Subject to change or removal.
func (irh interceptedResponseHandler) WriteHeader(statusCode int) {
	if irh.statusCode != 0 && (statusCode < 100 || statusCode >= 200) {
		irh.ResponseRecorder.WriteHeader(irh.statusCode)

		return
	}

	irh.ResponseRecorder.WriteHeader(statusCode)
}

// EXPERIMENTAL: Subject to change or removal.
func (ir Intercept) ServeHTTP(w http.ResponseWriter, r *http.Request, next kenginehttp.Handler) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	repl := r.Context().Value(kengine.ReplacerCtxKey).(*kengine.Replacer)
	rec := interceptedResponseHandler{replacer: repl}
	rec.ResponseRecorder = kenginehttp.NewResponseRecorder(w, buf, func(status int, header http.Header) bool {
		// see if any response handler is configured for this original response
		for i, rh := range ir.HandleResponse {
			if rh.Match != nil && !rh.Match.Match(status, header) {
				continue
			}
			rec.handler = rh
			rec.handlerIndex = i

			// if configured to only change the status code,
			// do that then stream
			if statusCodeStr := rh.StatusCode.String(); statusCodeStr != "" {
				sc, err := strconv.Atoi(repl.ReplaceAll(statusCodeStr, ""))
				if err != nil {
					rec.statusCode = http.StatusInternalServerError
				} else {
					rec.statusCode = sc
				}
			}

			return rec.statusCode == 0
		}

		return false
	})

	if err := next.ServeHTTP(rec, r); err != nil {
		return err
	}
	if !rec.Buffered() {
		return nil
	}

	// set up the replacer so that parts of the original response can be
	// used for routing decisions
	for field, value := range rec.Header() {
		repl.Set("http.intercept.header."+field, strings.Join(value, ","))
	}
	repl.Set("http.intercept.status_code", rec.Status())

	if c := ir.logger.Check(zapcore.DebugLevel, "handling response"); c != nil {
		c.Write(zap.Int("handler", rec.handlerIndex))
	}

	// pass the request through the response handler routes
	return rec.handler.Routes.Compile(next).ServeHTTP(w, r)
}

// UnmarshalKenginefile sets up the handler from Kenginefile tokens. Syntax:
//
//	intercept [<matcher>] {
//	    # intercept original responses
//	    @name {
//	        status <code...>
//	        header <field> [<value>]
//	    }
//	    replace_status [<matcher>] <status_code>
//	    handle_response [<matcher>] {
//	        <directives...>
//	    }
//	}
//
// The FinalizeUnmarshalKenginefile method should be called after this
// to finalize parsing of "handle_response" blocks, if possible.
//
// EXPERIMENTAL: Subject to change or removal.
func (i *Intercept) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	// collect the response matchers defined as subdirectives
	// prefixed with "@" for use with "handle_response" blocks
	i.responseMatchers = make(map[string]kenginehttp.ResponseMatcher)

	d.Next() // consume the directive name
	for d.NextBlock(0) {
		// if the subdirective has an "@" prefix then we
		// parse it as a response matcher for use with "handle_response"
		if strings.HasPrefix(d.Val(), matcherPrefix) {
			err := kenginehttp.ParseNamedResponseMatcher(d.NewFromNextSegment(), i.responseMatchers)
			if err != nil {
				return err
			}
			continue
		}

		switch d.Val() {
		case "handle_response":
			// delegate the parsing of handle_response to the caller,
			// since we need the httpkenginefile.Helper to parse subroutes.
			// See h.FinalizeUnmarshalKenginefile
			i.handleResponseSegments = append(i.handleResponseSegments, d.NewFromNextSegment())

		case "replace_status":
			args := d.RemainingArgs()
			if len(args) != 1 && len(args) != 2 {
				return d.Errf("must have one or two arguments: an optional response matcher, and a status code")
			}

			responseHandler := kenginehttp.ResponseHandler{}

			if len(args) == 2 {
				if !strings.HasPrefix(args[0], matcherPrefix) {
					return d.Errf("must use a named response matcher, starting with '@'")
				}
				foundMatcher, ok := i.responseMatchers[args[0]]
				if !ok {
					return d.Errf("no named response matcher defined with name '%s'", args[0][1:])
				}
				responseHandler.Match = &foundMatcher
				responseHandler.StatusCode = kenginehttp.WeakString(args[1])
			} else if len(args) == 1 {
				responseHandler.StatusCode = kenginehttp.WeakString(args[0])
			}

			// make sure there's no block, cause it doesn't make sense
			if nesting := d.Nesting(); d.NextBlock(nesting) {
				return d.Errf("cannot define routes for 'replace_status', use 'handle_response' instead.")
			}

			i.HandleResponse = append(
				i.HandleResponse,
				responseHandler,
			)

		default:
			return d.Errf("unrecognized subdirective %s", d.Val())
		}
	}

	return nil
}

// FinalizeUnmarshalKenginefile finalizes the Kenginefile parsing which
// requires having an httpkenginefile.Helper to function, to parse subroutes.
//
// EXPERIMENTAL: Subject to change or removal.
func (i *Intercept) FinalizeUnmarshalKenginefile(helper httpkenginefile.Helper) error {
	for _, d := range i.handleResponseSegments {
		// consume the "handle_response" token
		d.Next()
		args := d.RemainingArgs()

		// TODO: Remove this check at some point in the future
		if len(args) == 2 {
			return d.Errf("configuring 'handle_response' for status code replacement is no longer supported. Use 'replace_status' instead.")
		}

		if len(args) > 1 {
			return d.Errf("too many arguments for 'handle_response': %s", args)
		}

		var matcher *kenginehttp.ResponseMatcher
		if len(args) == 1 {
			// the first arg should always be a matcher.
			if !strings.HasPrefix(args[0], matcherPrefix) {
				return d.Errf("must use a named response matcher, starting with '@'")
			}

			foundMatcher, ok := i.responseMatchers[args[0]]
			if !ok {
				return d.Errf("no named response matcher defined with name '%s'", args[0][1:])
			}
			matcher = &foundMatcher
		}

		// parse the block as routes
		handler, err := httpkenginefile.ParseSegmentAsSubroute(helper.WithDispenser(d.NewFromNextSegment()))
		if err != nil {
			return err
		}
		subroute, ok := handler.(*kenginehttp.Subroute)
		if !ok {
			return helper.Errf("segment was not parsed as a subroute")
		}
		i.HandleResponse = append(
			i.HandleResponse,
			kenginehttp.ResponseHandler{
				Match:  matcher,
				Routes: subroute.Routes,
			},
		)
	}

	// move the handle_response entries without a matcher to the end.
	// we can't use sort.SliceStable because it will reorder the rest of the
	// entries which may be undesirable because we don't have a good
	// heuristic to use for sorting.
	withoutMatchers := []kenginehttp.ResponseHandler{}
	withMatchers := []kenginehttp.ResponseHandler{}
	for _, hr := range i.HandleResponse {
		if hr.Match == nil {
			withoutMatchers = append(withoutMatchers, hr)
		} else {
			withMatchers = append(withMatchers, hr)
		}
	}
	i.HandleResponse = append(withMatchers, withoutMatchers...)

	// clean up the bits we only needed for adapting
	i.handleResponseSegments = nil
	i.responseMatchers = nil

	return nil
}

const matcherPrefix = "@"

func parseKenginefile(helper httpkenginefile.Helper) (kenginehttp.MiddlewareHandler, error) {
	var ir Intercept
	if err := ir.UnmarshalKenginefile(helper.Dispenser); err != nil {
		return nil, err
	}

	if err := ir.FinalizeUnmarshalKenginefile(helper); err != nil {
		return nil, err
	}

	return ir, nil
}

// Interface guards
var (
	_ kengine.Provisioner           = (*Intercept)(nil)
	_ kenginefile.Unmarshaler       = (*Intercept)(nil)
	_ kenginehttp.MiddlewareHandler = (*Intercept)(nil)
)
