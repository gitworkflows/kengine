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

package logging

import (
	"github.com/khulnasoft/kengine/v2/kengineconfig/httpkenginefile"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func init() {
	httpkenginefile.RegisterHandlerDirective("log_append", parseKenginefile)
}

// parseKenginefile sets up the log_append handler from Kenginefile tokens. Syntax:
//
//	log_append [<matcher>] <key> <value>
func parseKenginefile(h httpkenginefile.Helper) (kenginehttp.MiddlewareHandler, error) {
	handler := new(LogAppend)
	err := handler.UnmarshalKenginefile(h.Dispenser)
	return handler, err
}

// UnmarshalKenginefile implements kenginefile.Unmarshaler.
func (h *LogAppend) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume directive name
	if !d.NextArg() {
		return d.ArgErr()
	}
	h.Key = d.Val()
	if !d.NextArg() {
		return d.ArgErr()
	}
	h.Value = d.Val()
	return nil
}

// Interface guards
var (
	_ kenginefile.Unmarshaler = (*LogAppend)(nil)
)
