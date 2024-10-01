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

package rewrite

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig"
	"github.com/khulnasoft/kengine/v2/kengineconfig/httpkenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func init() {
	httpkenginefile.RegisterDirective("rewrite", parseKenginefileRewrite)
	httpkenginefile.RegisterHandlerDirective("method", parseKenginefileMethod)
	httpkenginefile.RegisterHandlerDirective("uri", parseKenginefileURI)
	httpkenginefile.RegisterDirective("handle_path", parseKenginefileHandlePath)
}

// parseKenginefileRewrite sets up a basic rewrite handler from Kenginefile tokens. Syntax:
//
//	rewrite [<matcher>] <to>
//
// Only URI components which are given in <to> will be set in the resulting URI.
// See the docs for the rewrite handler for more information.
func parseKenginefileRewrite(h httpkenginefile.Helper) ([]httpkenginefile.ConfigValue, error) {
	h.Next() // consume directive name

	// count the tokens to determine what to do
	argsCount := h.CountRemainingArgs()
	if argsCount == 0 {
		return nil, h.Errf("too few arguments; must have at least a rewrite URI")
	}
	if argsCount > 2 {
		return nil, h.Errf("too many arguments; should only be a matcher and a URI")
	}

	// with only one arg, assume it's a rewrite URI with no matcher token
	if argsCount == 1 {
		if !h.NextArg() {
			return nil, h.ArgErr()
		}
		return h.NewRoute(nil, Rewrite{URI: h.Val()}), nil
	}

	// parse the matcher token into a matcher set
	userMatcherSet, err := h.ExtractMatcherSet()
	if err != nil {
		return nil, err
	}
	h.Next() // consume directive name again, matcher parsing does a reset
	h.Next() // advance to the rewrite URI

	return h.NewRoute(userMatcherSet, Rewrite{URI: h.Val()}), nil
}

// parseKenginefileMethod sets up a basic method rewrite handler from Kenginefile tokens. Syntax:
//
//	method [<matcher>] <method>
func parseKenginefileMethod(h httpkenginefile.Helper) (kenginehttp.MiddlewareHandler, error) {
	h.Next() // consume directive name
	if !h.NextArg() {
		return nil, h.ArgErr()
	}
	if h.NextArg() {
		return nil, h.ArgErr()
	}
	return Rewrite{Method: h.Val()}, nil
}

// parseKenginefileURI sets up a handler for manipulating (but not "rewriting") the
// URI from Kenginefile tokens. Syntax:
//
//	uri [<matcher>] strip_prefix|strip_suffix|replace|path_regexp <target> [<replacement> [<limit>]]
//
// If strip_prefix or strip_suffix are used, then <target> will be stripped
// only if it is the beginning or the end, respectively, of the URI path. If
// replace is used, then <target> will be replaced with <replacement> across
// the whole URI, up to <limit> times (or unlimited if unspecified). If
// path_regexp is used, then regular expression replacements will be performed
// on the path portion of the URI (and a limit cannot be set).
func parseKenginefileURI(h httpkenginefile.Helper) (kenginehttp.MiddlewareHandler, error) {
	h.Next() // consume directive name

	args := h.RemainingArgs()
	if len(args) < 1 {
		return nil, h.ArgErr()
	}

	var rewr Rewrite

	switch args[0] {
	case "strip_prefix":
		if len(args) != 2 {
			return nil, h.ArgErr()
		}
		rewr.StripPathPrefix = args[1]
		if !strings.HasPrefix(rewr.StripPathPrefix, "/") {
			rewr.StripPathPrefix = "/" + rewr.StripPathPrefix
		}

	case "strip_suffix":
		if len(args) != 2 {
			return nil, h.ArgErr()
		}
		rewr.StripPathSuffix = args[1]

	case "replace":
		var find, replace, lim string
		switch len(args) {
		case 4:
			lim = args[3]
			fallthrough
		case 3:
			find = args[1]
			replace = args[2]
		default:
			return nil, h.ArgErr()
		}

		var limInt int
		if lim != "" {
			var err error
			limInt, err = strconv.Atoi(lim)
			if err != nil {
				return nil, h.Errf("limit must be an integer; invalid: %v", err)
			}
		}

		rewr.URISubstring = append(rewr.URISubstring, substrReplacer{
			Find:    find,
			Replace: replace,
			Limit:   limInt,
		})

	case "path_regexp":
		if len(args) != 3 {
			return nil, h.ArgErr()
		}
		find, replace := args[1], args[2]
		rewr.PathRegexp = append(rewr.PathRegexp, &regexReplacer{
			Find:    find,
			Replace: replace,
		})

	case "query":
		if len(args) > 4 {
			return nil, h.ArgErr()
		}
		rewr.Query = &queryOps{}
		var hasArgs bool
		if len(args) > 1 {
			hasArgs = true
			err := applyQueryOps(h, rewr.Query, args[1:])
			if err != nil {
				return nil, err
			}
		}

		for h.NextBlock(0) {
			if hasArgs {
				return nil, h.Err("Cannot specify uri query rewrites in both argument and block")
			}
			queryArgs := []string{h.Val()}
			queryArgs = append(queryArgs, h.RemainingArgs()...)
			err := applyQueryOps(h, rewr.Query, queryArgs)
			if err != nil {
				return nil, err
			}
		}

	default:
		return nil, h.Errf("unrecognized URI manipulation '%s'", args[0])
	}
	return rewr, nil
}

func applyQueryOps(h httpkenginefile.Helper, qo *queryOps, args []string) error {
	key := args[0]
	switch {
	case strings.HasPrefix(key, "-"):
		if len(args) != 1 {
			return h.ArgErr()
		}
		qo.Delete = append(qo.Delete, strings.TrimLeft(key, "-"))

	case strings.HasPrefix(key, "+"):
		if len(args) != 2 {
			return h.ArgErr()
		}
		param := strings.TrimLeft(key, "+")
		qo.Add = append(qo.Add, queryOpsArguments{Key: param, Val: args[1]})

	case strings.Contains(key, ">"):
		if len(args) != 1 {
			return h.ArgErr()
		}
		renameValKey := strings.Split(key, ">")
		qo.Rename = append(qo.Rename, queryOpsArguments{Key: renameValKey[0], Val: renameValKey[1]})

	case len(args) == 3:
		qo.Replace = append(qo.Replace, &queryOpsReplacement{Key: key, SearchRegexp: args[1], Replace: args[2]})

	default:
		if len(args) != 2 {
			return h.ArgErr()
		}
		qo.Set = append(qo.Set, queryOpsArguments{Key: key, Val: args[1]})
	}
	return nil
}

// parseKenginefileHandlePath parses the handle_path directive. Syntax:
//
//	handle_path [<matcher>] {
//	    <directives...>
//	}
//
// Only path matchers (with a `/` prefix) are supported as this is a shortcut
// for the handle directive with a strip_prefix rewrite.
func parseKenginefileHandlePath(h httpkenginefile.Helper) ([]httpkenginefile.ConfigValue, error) {
	h.Next() // consume directive name

	// there must be a path matcher
	if !h.NextArg() {
		return nil, h.ArgErr()
	}

	// read the prefix to strip
	path := h.Val()
	if !strings.HasPrefix(path, "/") {
		return nil, h.Errf("path matcher must begin with '/', got %s", path)
	}

	// we only want to strip what comes before the '/' if
	// the user specified it (e.g. /api/* should only strip /api)
	var stripPath string
	if strings.HasSuffix(path, "/*") {
		stripPath = path[:len(path)-2]
	} else if strings.HasSuffix(path, "*") {
		stripPath = path[:len(path)-1]
	} else {
		stripPath = path
	}

	// the ParseSegmentAsSubroute function expects the cursor
	// to be at the token just before the block opening,
	// so we need to rewind because we already read past it
	h.Reset()
	h.Next()

	// parse the block contents as a subroute handler
	handler, err := httpkenginefile.ParseSegmentAsSubroute(h)
	if err != nil {
		return nil, err
	}
	subroute, ok := handler.(*kenginehttp.Subroute)
	if !ok {
		return nil, h.Errf("segment was not parsed as a subroute")
	}

	// make a matcher on the path and everything below it
	pathMatcher := kengine.ModuleMap{
		"path": h.JSON(kenginehttp.MatchPath{path}),
	}

	// build a route with a rewrite handler to strip the path prefix
	route := kenginehttp.Route{
		HandlersRaw: []json.RawMessage{
			kengineconfig.JSONModuleObject(Rewrite{
				StripPathPrefix: stripPath,
			}, "handler", "rewrite", nil),
		},
	}

	// prepend the route to the subroute
	subroute.Routes = append([]kenginehttp.Route{route}, subroute.Routes...)

	// build and return a route from the subroute
	return h.NewRoute(pathMatcher, subroute), nil
}
