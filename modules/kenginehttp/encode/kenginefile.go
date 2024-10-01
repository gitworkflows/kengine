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

package encode

import (
	"strconv"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig"
	"github.com/khulnasoft/kengine/v2/kengineconfig/httpkenginefile"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func init() {
	httpkenginefile.RegisterHandlerDirective("encode", parseKenginefile)
}

func parseKenginefile(h httpkenginefile.Helper) (kenginehttp.MiddlewareHandler, error) {
	enc := new(Encode)
	err := enc.UnmarshalKenginefile(h.Dispenser)
	if err != nil {
		return nil, err
	}
	return enc, nil
}

// UnmarshalKenginefile sets up the handler from Kenginefile tokens. Syntax:
//
//	encode [<matcher>] <formats...> {
//	    gzip           [<level>]
//	    zstd
//	    minimum_length <length>
//	    # response matcher block
//	    match {
//	        status <code...>
//	        header <field> [<value>]
//	    }
//	    # or response matcher single line syntax
//	    match [header <field> [<value>]] | [status <code...>]
//	}
//
// Specifying the formats on the first line will use those formats' defaults.
func (enc *Encode) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume directive name

	prefer := []string{}
	for _, arg := range d.RemainingArgs() {
		mod, err := kengine.GetModule("http.encoders." + arg)
		if err != nil {
			return d.Errf("finding encoder module '%s': %v", mod, err)
		}
		encoding, ok := mod.New().(Encoding)
		if !ok {
			return d.Errf("module %s is not an HTTP encoding", mod)
		}
		if enc.EncodingsRaw == nil {
			enc.EncodingsRaw = make(kengine.ModuleMap)
		}
		enc.EncodingsRaw[arg] = kengineconfig.JSON(encoding, nil)
		prefer = append(prefer, arg)
	}

	responseMatchers := make(map[string]kenginehttp.ResponseMatcher)
	for d.NextBlock(0) {
		switch d.Val() {
		case "minimum_length":
			if !d.NextArg() {
				return d.ArgErr()
			}
			minLength, err := strconv.Atoi(d.Val())
			if err != nil {
				return err
			}
			enc.MinLength = minLength
		case "match":
			err := kenginehttp.ParseNamedResponseMatcher(d.NewFromNextSegment(), responseMatchers)
			if err != nil {
				return err
			}
			matcher := responseMatchers["match"]
			enc.Matcher = &matcher
		default:
			name := d.Val()
			modID := "http.encoders." + name
			unm, err := kenginefile.UnmarshalModule(d, modID)
			if err != nil {
				return err
			}
			encoding, ok := unm.(Encoding)
			if !ok {
				return d.Errf("module %s is not an HTTP encoding; is %T", modID, unm)
			}
			if enc.EncodingsRaw == nil {
				enc.EncodingsRaw = make(kengine.ModuleMap)
			}
			enc.EncodingsRaw[name] = kengineconfig.JSON(encoding, nil)
			prefer = append(prefer, name)
		}
	}

	// use the order in which the encoders were defined.
	enc.Prefer = prefer

	return nil
}

// Interface guard
var _ kenginefile.Unmarshaler = (*Encode)(nil)
