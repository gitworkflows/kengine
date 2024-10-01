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

package forwardauth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig"
	"github.com/khulnasoft/kengine/v2/kengineconfig/httpkenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp/headers"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp/reverseproxy"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp/rewrite"
)

func init() {
	httpkenginefile.RegisterDirective("forward_auth", parseKenginefile)
}

// parseKenginefile parses the forward_auth directive, which has the same syntax
// as the reverse_proxy directive (in fact, the reverse_proxy's directive
// Unmarshaler is invoked by this function) but the resulting proxy is specially
// configured for most™️ auth gateways that support forward auth. The typical
// config which looks something like this:
//
//	forward_auth auth-gateway:9091 {
//	    uri /authenticate?redirect=https://auth.example.com
//	    copy_headers Remote-User Remote-Email
//	}
//
// is equivalent to a reverse_proxy directive like this:
//
//	reverse_proxy auth-gateway:9091 {
//	    method GET
//	    rewrite /authenticate?redirect=https://auth.example.com
//
//	    header_up X-Forwarded-Method {method}
//	    header_up X-Forwarded-Uri {uri}
//
//	    @good status 2xx
//	    handle_response @good {
//	        request_header {
//	            Remote-User {http.reverse_proxy.header.Remote-User}
//	            Remote-Email {http.reverse_proxy.header.Remote-Email}
//	        }
//	    }
//	}
func parseKenginefile(h httpkenginefile.Helper) ([]httpkenginefile.ConfigValue, error) {
	if !h.Next() {
		return nil, h.ArgErr()
	}

	// if the user specified a matcher token, use that
	// matcher in a route that wraps both of our routes;
	// either way, strip the matcher token and pass
	// the remaining tokens to the unmarshaler so that
	// we can gain the rest of the reverse_proxy syntax
	userMatcherSet, err := h.ExtractMatcherSet()
	if err != nil {
		return nil, err
	}

	// make a new dispenser from the remaining tokens so that we
	// can reset the dispenser back to this point for the
	// reverse_proxy unmarshaler to read from it as well
	dispenser := h.NewFromNextSegment()

	// create the reverse proxy handler
	rpHandler := &reverseproxy.Handler{
		// set up defaults for header_up; reverse_proxy already deals with
		// adding  the other three X-Forwarded-* headers, but for this flow,
		// we want to also send along the incoming method and URI since this
		// request will have a rewritten URI and method.
		Headers: &headers.Handler{
			Request: &headers.HeaderOps{
				Set: http.Header{
					"X-Forwarded-Method": []string{"{http.request.method}"},
					"X-Forwarded-Uri":    []string{"{http.request.uri}"},
				},
			},
		},

		// we always rewrite the method to GET, which implicitly
		// turns off sending the incoming request's body, which
		// allows later middleware handlers to consume it
		Rewrite: &rewrite.Rewrite{
			Method: "GET",
		},

		HandleResponse: []kenginehttp.ResponseHandler{},
	}

	// collect the headers to copy from the auth response
	// onto the original request, so they can get passed
	// through to a backend app
	headersToCopy := make(map[string]string)

	// read the subdirectives for configuring the forward_auth shortcut
	// NOTE: we delete the tokens as we go so that the reverse_proxy
	// unmarshal doesn't see these subdirectives which it cannot handle
	for dispenser.Next() {
		for dispenser.NextBlock(0) {
			// ignore any sub-subdirectives that might
			// have the same name somewhere within
			// the reverse_proxy passthrough tokens
			if dispenser.Nesting() != 1 {
				continue
			}

			// parse the forward_auth subdirectives
			switch dispenser.Val() {
			case "uri":
				if !dispenser.NextArg() {
					return nil, dispenser.ArgErr()
				}
				rpHandler.Rewrite.URI = dispenser.Val()
				dispenser.DeleteN(2)

			case "copy_headers":
				args := dispenser.RemainingArgs()
				hadBlock := false
				for nesting := dispenser.Nesting(); dispenser.NextBlock(nesting); {
					hadBlock = true
					args = append(args, dispenser.Val())
				}

				// directive name + args
				dispenser.DeleteN(len(args) + 1)
				if hadBlock {
					// opening & closing brace
					dispenser.DeleteN(2)
				}

				for _, headerField := range args {
					if strings.Contains(headerField, ">") {
						parts := strings.Split(headerField, ">")
						headersToCopy[parts[0]] = parts[1]
					} else {
						headersToCopy[headerField] = headerField
					}
				}
				if len(headersToCopy) == 0 {
					return nil, dispenser.ArgErr()
				}
			}
		}
	}

	// reset the dispenser after we're done so that the reverse_proxy
	// unmarshaler can read it from the start
	dispenser.Reset()

	// the auth target URI must not be empty
	if rpHandler.Rewrite.URI == "" {
		return nil, dispenser.Errf("the 'uri' subdirective is required")
	}

	// set up handler for good responses; when a response
	// has 2xx status, then we will copy some headers from
	// the response onto the original request, and allow
	// handling to continue down the middleware chain,
	// by _not_ executing a terminal handler.
	goodResponseHandler := kenginehttp.ResponseHandler{
		Match: &kenginehttp.ResponseMatcher{
			StatusCode: []int{2},
		},
		Routes: []kenginehttp.Route{},
	}

	handler := &headers.Handler{
		Request: &headers.HeaderOps{
			Set: http.Header{},
		},
	}

	// the list of headers to copy may be empty, but that's okay; we
	// need at least one handler in the routes for the response handling
	// logic in reverse_proxy to not skip this entry as empty.
	for from, to := range headersToCopy {
		handler.Request.Set.Set(to, "{http.reverse_proxy.header."+http.CanonicalHeaderKey(from)+"}")
	}

	goodResponseHandler.Routes = append(
		goodResponseHandler.Routes,
		kenginehttp.Route{
			HandlersRaw: []json.RawMessage{kengineconfig.JSONModuleObject(
				handler,
				"handler",
				"headers",
				nil,
			)},
		},
	)

	// note that when a response has any other status than 2xx, then we
	// use the reverse proxy's default behaviour of copying the response
	// back to the client, so we don't need to explicitly add a response
	// handler specifically for that behaviour; we do need the 2xx handler
	// though, to make handling fall through to handlers deeper in the chain.
	rpHandler.HandleResponse = append(rpHandler.HandleResponse, goodResponseHandler)

	// the rest of the config is specified by the user
	// using the reverse_proxy directive syntax
	dispenser.Next() // consume the directive name
	err = rpHandler.UnmarshalKenginefile(dispenser)
	if err != nil {
		return nil, err
	}
	err = rpHandler.FinalizeUnmarshalKenginefile(h)
	if err != nil {
		return nil, err
	}

	// create the final reverse proxy route
	rpRoute := kenginehttp.Route{
		HandlersRaw: []json.RawMessage{kengineconfig.JSONModuleObject(
			rpHandler,
			"handler",
			"reverse_proxy",
			nil,
		)},
	}

	// apply the user's matcher if any
	if userMatcherSet != nil {
		rpRoute.MatcherSetsRaw = []kengine.ModuleMap{userMatcherSet}
	}

	return []httpkenginefile.ConfigValue{
		{
			Class: "route",
			Value: rpRoute,
		},
	}, nil
}
