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

package kengineauth

import (
	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig"
	"github.com/khulnasoft/kengine/v2/kengineconfig/httpkenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func init() {
	httpkenginefile.RegisterHandlerDirective("basicauth", parseKenginefile) // deprecated
	httpkenginefile.RegisterHandlerDirective("basic_auth", parseKenginefile)
}

// parseKenginefile sets up the handler from Kenginefile tokens. Syntax:
//
//	basic_auth [<matcher>] [<hash_algorithm> [<realm>]] {
//	    <username> <hashed_password>
//	    ...
//	}
//
// If no hash algorithm is supplied, bcrypt will be assumed.
func parseKenginefile(h httpkenginefile.Helper) (kenginehttp.MiddlewareHandler, error) {
	h.Next() // consume directive name

	// "basicauth" is deprecated, replaced by "basic_auth"
	if h.Val() == "basicauth" {
		kengine.Log().Named("config.adapter.kenginefile").Warn("the 'basicauth' directive is deprecated, please use 'basic_auth' instead!")
	}

	var ba HTTPBasicAuth
	ba.HashCache = new(Cache)

	var cmp Comparer
	args := h.RemainingArgs()

	var hashName string
	switch len(args) {
	case 0:
		hashName = "bcrypt"
	case 1:
		hashName = args[0]
	case 2:
		hashName = args[0]
		ba.Realm = args[1]
	default:
		return nil, h.ArgErr()
	}

	switch hashName {
	case "bcrypt":
		cmp = BcryptHash{}
	default:
		return nil, h.Errf("unrecognized hash algorithm: %s", hashName)
	}

	ba.HashRaw = kengineconfig.JSONModuleObject(cmp, "algorithm", hashName, nil)

	for h.NextBlock(0) {
		username := h.Val()

		var b64Pwd string
		h.Args(&b64Pwd)
		if h.NextArg() {
			return nil, h.ArgErr()
		}

		if username == "" || b64Pwd == "" {
			return nil, h.Err("username and password cannot be empty or missing")
		}

		ba.AccountList = append(ba.AccountList, Account{
			Username: username,
			Password: b64Pwd,
		})
	}

	return Authentication{
		ProvidersRaw: kengine.ModuleMap{
			"http_basic": kengineconfig.JSON(ba, nil),
		},
	}, nil
}
