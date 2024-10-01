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

package headers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func TestHandler(t *testing.T) {
	for i, tc := range []struct {
		handler            Handler
		reqHeader          http.Header
		respHeader         http.Header
		respStatusCode     int
		expectedReqHeader  http.Header
		expectedRespHeader http.Header
	}{
		{
			handler: Handler{
				Request: &HeaderOps{
					Add: http.Header{
						"Expose-Secrets": []string{"always"},
					},
				},
			},
			reqHeader: http.Header{
				"Expose-Secrets": []string{"i'm serious"},
			},
			expectedReqHeader: http.Header{
				"Expose-Secrets": []string{"i'm serious", "always"},
			},
		},
		{
			handler: Handler{
				Request: &HeaderOps{
					Set: http.Header{
						"Who-Wins": []string{"batman"},
					},
				},
			},
			reqHeader: http.Header{
				"Who-Wins": []string{"joker"},
			},
			expectedReqHeader: http.Header{
				"Who-Wins": []string{"batman"},
			},
		},
		{
			handler: Handler{
				Request: &HeaderOps{
					Delete: []string{"Kick-Me"},
				},
			},
			reqHeader: http.Header{
				"Kick-Me": []string{"if you can"},
				"Keep-Me": []string{"i swear i'm innocent"},
			},
			expectedReqHeader: http.Header{
				"Keep-Me": []string{"i swear i'm innocent"},
			},
		},
		{
			handler: Handler{
				Request: &HeaderOps{
					Delete: []string{
						"*-suffix",
						"prefix-*",
						"*_*",
					},
				},
			},
			reqHeader: http.Header{
				"Header-Suffix": []string{"lalala"},
				"Prefix-Test":   []string{"asdf"},
				"Host_Header":   []string{"silly django... sigh"}, // see issue #4830
				"Keep-Me":       []string{"foofoofoo"},
			},
			expectedReqHeader: http.Header{
				"Keep-Me": []string{"foofoofoo"},
			},
		},
		{
			handler: Handler{
				Request: &HeaderOps{
					Replace: map[string][]Replacement{
						"Best-Server": {
							Replacement{
								Search:  "NGINX",
								Replace: "the Kengine web server",
							},
							Replacement{
								SearchRegexp: `Apache(\d+)`,
								Replace:      "Kengine",
							},
						},
					},
				},
			},
			reqHeader: http.Header{
				"Best-Server": []string{"it's NGINX, undoubtedly", "I love Apache2"},
			},
			expectedReqHeader: http.Header{
				"Best-Server": []string{"it's the Kengine web server, undoubtedly", "I love Kengine"},
			},
		},
		{
			handler: Handler{
				Response: &RespHeaderOps{
					Require: &kenginehttp.ResponseMatcher{
						Headers: http.Header{
							"Cache-Control": nil,
						},
					},
					HeaderOps: &HeaderOps{
						Add: http.Header{
							"Cache-Control": []string{"no-cache"},
						},
					},
				},
			},
			respHeader: http.Header{},
			expectedRespHeader: http.Header{
				"Cache-Control": []string{"no-cache"},
			},
		},
		{
			handler: Handler{
				Response: &RespHeaderOps{
					Require: &kenginehttp.ResponseMatcher{
						Headers: http.Header{
							"Cache-Control": []string{"no-cache"},
						},
					},
					HeaderOps: &HeaderOps{
						Delete: []string{"Cache-Control"},
					},
				},
			},
			respHeader: http.Header{
				"Cache-Control": []string{"no-cache"},
			},
			expectedRespHeader: http.Header{},
		},
		{
			handler: Handler{
				Response: &RespHeaderOps{
					Require: &kenginehttp.ResponseMatcher{
						StatusCode: []int{5},
					},
					HeaderOps: &HeaderOps{
						Add: http.Header{
							"Fail-5xx": []string{"true"},
						},
					},
				},
			},
			respStatusCode: 503,
			respHeader:     http.Header{},
			expectedRespHeader: http.Header{
				"Fail-5xx": []string{"true"},
			},
		},
		{
			handler: Handler{
				Request: &HeaderOps{
					Replace: map[string][]Replacement{
						"Case-Insensitive": {
							Replacement{
								Search:  "issue4330",
								Replace: "issue #4330",
							},
						},
					},
				},
			},
			reqHeader: http.Header{
				"case-insensitive": []string{"issue4330"},
				"Other-Header":     []string{"issue4330"},
			},
			expectedReqHeader: http.Header{
				"case-insensitive": []string{"issue #4330"},
				"Other-Header":     []string{"issue4330"},
			},
		},
	} {
		rr := httptest.NewRecorder()

		req := &http.Request{Header: tc.reqHeader}
		repl := kengine.NewReplacer()
		ctx := context.WithValue(req.Context(), kengine.ReplacerCtxKey, repl)
		req = req.WithContext(ctx)

		tc.handler.Provision(kengine.Context{})

		next := nextHandler(func(w http.ResponseWriter, r *http.Request) error {
			for k, hdrs := range tc.respHeader {
				for _, v := range hdrs {
					w.Header().Add(k, v)
				}
			}

			status := 200
			if tc.respStatusCode != 0 {
				status = tc.respStatusCode
			}
			w.WriteHeader(status)

			if tc.expectedReqHeader != nil && !reflect.DeepEqual(r.Header, tc.expectedReqHeader) {
				return fmt.Errorf("expected request header %v, got %v", tc.expectedReqHeader, r.Header)
			}

			return nil
		})

		if err := tc.handler.ServeHTTP(rr, req, next); err != nil {
			t.Errorf("Test %d: %v", i, err)
			continue
		}

		actual := rr.Header()
		if tc.expectedRespHeader != nil && !reflect.DeepEqual(actual, tc.expectedRespHeader) {
			t.Errorf("Test %d: expected response header %v, got %v", i, tc.expectedRespHeader, actual)
			continue
		}
	}
}

type nextHandler func(http.ResponseWriter, *http.Request) error

func (f nextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	return f(w, r)
}
