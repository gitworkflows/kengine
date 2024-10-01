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

package fileserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/khulnasoft-lab/certmagic"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	kenginecmd "github.com/khulnasoft/kengine/v2/cmd"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp/encode"
	kenginetpl "github.com/khulnasoft/kengine/v2/modules/kenginehttp/templates"
)

func init() {
	kenginecmd.RegisterCommand(kenginecmd.Command{
		Name:  "file-server",
		Usage: "[--domain <example.com>] [--root <path>] [--listen <addr>] [--browse] [--reveal-symlinks] [--access-log] [--precompressed]",
		Short: "Spins up a production-ready file server",
		Long: `
A simple but production-ready file server. Useful for quick deployments,
demos, and development.

The listener's socket address can be customized with the --listen flag.

If a domain name is specified with --domain, the default listener address
will be changed to the HTTPS port and the server will use HTTPS. If using
a public domain, ensure A/AAAA records are properly configured before
using this option.

By default, Zstandard and Gzip compression are enabled. Use --no-compress
to disable compression.

If --browse is enabled, requests for folders without an index file will
respond with a file listing.`,
		CobraFunc: func(cmd *cobra.Command) {
			cmd.Flags().StringP("domain", "d", "", "Domain name at which to serve the files")
			cmd.Flags().StringP("root", "r", "", "The path to the root of the site")
			cmd.Flags().StringP("listen", "l", "", "The address to which to bind the listener")
			cmd.Flags().BoolP("browse", "b", false, "Enable directory browsing")
			cmd.Flags().BoolP("reveal-symlinks", "", false, "Show symlink paths when browse is enabled.")
			cmd.Flags().BoolP("templates", "t", false, "Enable template rendering")
			cmd.Flags().BoolP("access-log", "a", false, "Enable the access log")
			cmd.Flags().BoolP("debug", "v", false, "Enable verbose debug logs")
			cmd.Flags().BoolP("no-compress", "", false, "Disable Zstandard and Gzip compression")
			cmd.Flags().StringSliceP("precompressed", "p", []string{}, "Specify precompression file extensions. Compression preference implied from flag order.")
			cmd.RunE = kenginecmd.WrapCommandFuncForCobra(cmdFileServer)
			cmd.AddCommand(&cobra.Command{
				Use:     "export-template",
				Short:   "Exports the default file browser template",
				Example: "kengine file-server export-template > browse.html",
				RunE: func(cmd *cobra.Command, args []string) error {
					_, err := io.WriteString(os.Stdout, BrowseTemplate)
					return err
				},
			})
		},
	})
}

func cmdFileServer(fs kenginecmd.Flags) (int, error) {
	kengine.TrapSignals()

	domain := fs.String("domain")
	root := fs.String("root")
	listen := fs.String("listen")
	browse := fs.Bool("browse")
	templates := fs.Bool("templates")
	accessLog := fs.Bool("access-log")
	debug := fs.Bool("debug")
	revealSymlinks := fs.Bool("reveal-symlinks")
	compress := !fs.Bool("no-compress")
	precompressed, err := fs.GetStringSlice("precompressed")
	if err != nil {
		return kengine.ExitCodeFailedStartup, fmt.Errorf("invalid precompressed flag: %v", err)
	}
	var handlers []json.RawMessage

	if compress {
		zstd, err := kengine.GetModule("http.encoders.zstd")
		if err != nil {
			return kengine.ExitCodeFailedStartup, err
		}

		gzip, err := kengine.GetModule("http.encoders.gzip")
		if err != nil {
			return kengine.ExitCodeFailedStartup, err
		}

		handlers = append(handlers, kengineconfig.JSONModuleObject(encode.Encode{
			EncodingsRaw: kengine.ModuleMap{
				"zstd": kengineconfig.JSON(zstd.New(), nil),
				"gzip": kengineconfig.JSON(gzip.New(), nil),
			},
			Prefer: []string{"zstd", "gzip"},
		}, "handler", "encode", nil))
	}

	if templates {
		handler := kenginetpl.Templates{FileRoot: root}
		handlers = append(handlers, kengineconfig.JSONModuleObject(handler, "handler", "templates", nil))
	}

	handler := FileServer{Root: root}

	if len(precompressed) != 0 {
		// logic mirrors modules/kenginehttp/fileserver/kenginefile.go case "precompressed"
		var order []string
		for _, compression := range precompressed {
			modID := "http.precompressed." + compression
			mod, err := kengine.GetModule(modID)
			if err != nil {
				return kengine.ExitCodeFailedStartup, fmt.Errorf("getting module named '%s': %v", modID, err)
			}
			inst := mod.New()
			precompress, ok := inst.(encode.Precompressed)
			if !ok {
				return kengine.ExitCodeFailedStartup, fmt.Errorf("module %s is not a precompressor; is %T", modID, inst)
			}
			if handler.PrecompressedRaw == nil {
				handler.PrecompressedRaw = make(kengine.ModuleMap)
			}
			handler.PrecompressedRaw[compression] = kengineconfig.JSON(precompress, nil)
			order = append(order, compression)
		}
		handler.PrecompressedOrder = order
	}

	if browse {
		handler.Browse = &Browse{RevealSymlinks: revealSymlinks}
	}

	handlers = append(handlers, kengineconfig.JSONModuleObject(handler, "handler", "file_server", nil))

	route := kenginehttp.Route{HandlersRaw: handlers}

	if domain != "" {
		route.MatcherSetsRaw = []kengine.ModuleMap{
			{
				"host": kengineconfig.JSON(kenginehttp.MatchHost{domain}, nil),
			},
		}
	}

	server := &kenginehttp.Server{
		ReadHeaderTimeout: kengine.Duration(10 * time.Second),
		IdleTimeout:       kengine.Duration(30 * time.Second),
		MaxHeaderBytes:    1024 * 10,
		Routes:            kenginehttp.RouteList{route},
	}
	if listen == "" {
		if domain == "" {
			listen = ":80"
		} else {
			listen = ":" + strconv.Itoa(certmagic.HTTPSPort)
		}
	}
	server.Listen = []string{listen}
	if accessLog {
		server.Logs = &kenginehttp.ServerLogConfig{}
	}

	httpApp := kenginehttp.App{
		Servers: map[string]*kenginehttp.Server{"static": server},
	}

	var false bool
	cfg := &kengine.Config{
		Admin: &kengine.AdminConfig{
			Disabled: true,
			Config: &kengine.ConfigSettings{
				Persist: &false,
			},
		},
		AppsRaw: kengine.ModuleMap{
			"http": kengineconfig.JSON(httpApp, nil),
		},
	}

	if debug {
		cfg.Logging = &kengine.Logging{
			Logs: map[string]*kengine.CustomLog{
				"default": {
					BaseLog: kengine.BaseLog{Level: zap.DebugLevel.CapitalString()},
				},
			},
		}
	}

	err = kengine.Run(cfg)
	if err != nil {
		return kengine.ExitCodeFailedStartup, err
	}

	log.Printf("Kengine serving static files on %s", listen)

	select {}
}
