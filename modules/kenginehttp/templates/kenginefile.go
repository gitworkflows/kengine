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

package templates

import (
	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig"
	"github.com/khulnasoft/kengine/v2/kengineconfig/httpkenginefile"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func init() {
	httpkenginefile.RegisterHandlerDirective("templates", parseKenginefile)
}

// parseKenginefile sets up the handler from Kenginefile tokens. Syntax:
//
//	templates [<matcher>] {
//	    mime <types...>
//	    between <open_delim> <close_delim>
//	    root <path>
//	}
func parseKenginefile(h httpkenginefile.Helper) (kenginehttp.MiddlewareHandler, error) {
	h.Next() // consume directive name
	t := new(Templates)
	for h.NextBlock(0) {
		switch h.Val() {
		case "mime":
			t.MIMETypes = h.RemainingArgs()
			if len(t.MIMETypes) == 0 {
				return nil, h.ArgErr()
			}
		case "between":
			t.Delimiters = h.RemainingArgs()
			if len(t.Delimiters) != 2 {
				return nil, h.ArgErr()
			}
		case "root":
			if !h.Args(&t.FileRoot) {
				return nil, h.ArgErr()
			}
		case "extensions":
			if h.NextArg() {
				return nil, h.ArgErr()
			}
			if t.ExtensionsRaw != nil {
				return nil, h.Err("extensions already specified")
			}
			for nesting := h.Nesting(); h.NextBlock(nesting); {
				extensionModuleName := h.Val()
				modID := "http.handlers.templates.functions." + extensionModuleName
				unm, err := kenginefile.UnmarshalModule(h.Dispenser, modID)
				if err != nil {
					return nil, err
				}
				cf, ok := unm.(CustomFunctions)
				if !ok {
					return nil, h.Errf("module %s (%T) does not provide template functions", modID, unm)
				}
				if t.ExtensionsRaw == nil {
					t.ExtensionsRaw = make(kengine.ModuleMap)
				}
				t.ExtensionsRaw[extensionModuleName] = kengineconfig.JSON(cf, nil)
			}
		}
	}
	return t, nil
}
