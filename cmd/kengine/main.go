// The below line is required to enable post-quantum key agreement in Go 1.23
// by default without insisting on setting a minimum version of 1.23 in go.mod.
// See https://github.com/khulnasoft/kengine/issues/6540#issuecomment-2313094905
//go:debug tlskyber=1

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

// Package main is the entry point of the Kengine application.
// Most of Kengine's functionality is provided through modules,
// which can be plugged in by adding their import below.
//
// There is no need to modify the Kengine source code to customize your
// builds. You can easily build a custom Kengine with these simple steps:
//
//  1. Copy this file (main.go) into a new folder
//  2. Edit the imports below to include the modules you want plugged in
//  3. Run `go mod init kengine`
//  4. Run `go install` or `go build` - you now have a custom binary!
//
// Or you can use xkengine which does it all for you as a command:
// https://github.com/khulnasoft/xkengine
package main

import (
	kenginecmd "github.com/khulnasoft/kengine/v2/cmd"

	// plug in Kengine modules here
	_ "github.com/khulnasoft/kengine/v2/modules/standard"
)

func main() {
	kenginecmd.Main()
}
