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

// Package eventsconfig is for configuring kengineevents.App with the
// Kenginefile. This code can't be in the kengineevents package because
// the httpkenginefile package imports kenginehttp, which imports
// kengineevents: hence, it creates an import cycle.
package eventsconfig

import (
	"encoding/json"

	"github.com/khulnasoft/kengine/v2/kengineconfig"
	"github.com/khulnasoft/kengine/v2/kengineconfig/httpkenginefile"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kengineevents"
)

func init() {
	httpkenginefile.RegisterGlobalOption("events", parseApp)
}

// parseApp configures the "events" global option from Kenginefile to set up the events app.
// Syntax:
//
//	events {
//		on <event> <handler_module...>
//	}
//
// If <event> is *, then it will bind to all events.
func parseApp(d *kenginefile.Dispenser, _ any) (any, error) {
	d.Next() // consume option name
	app := new(kengineevents.App)
	for d.NextBlock(0) {
		switch d.Val() {
		case "on":
			if !d.NextArg() {
				return nil, d.ArgErr()
			}
			eventName := d.Val()
			if eventName == "*" {
				eventName = ""
			}

			if !d.NextArg() {
				return nil, d.ArgErr()
			}
			handlerName := d.Val()
			modID := "events.handlers." + handlerName
			unm, err := kenginefile.UnmarshalModule(d, modID)
			if err != nil {
				return nil, err
			}

			app.Subscriptions = append(app.Subscriptions, &kengineevents.Subscription{
				Events: []string{eventName},
				HandlersRaw: []json.RawMessage{
					kengineconfig.JSONModuleObject(unm, "handler", handlerName, nil),
				},
			})

		default:
			return nil, d.ArgErr()
		}
	}

	return httpkenginefile.App{
		Name:  "events",
		Value: kengineconfig.JSON(app, nil),
	}, nil
}
