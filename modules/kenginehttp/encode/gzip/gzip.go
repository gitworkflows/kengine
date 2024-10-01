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

package kenginegzip

import (
	"fmt"
	"strconv"

	"github.com/klauspost/compress/gzip"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp/encode"
)

func init() {
	kengine.RegisterModule(Gzip{})
}

// Gzip can create gzip encoders.
type Gzip struct {
	Level int `json:"level,omitempty"`
}

// KengineModule returns the Kengine module information.
func (Gzip) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.encoders.gzip",
		New: func() kengine.Module { return new(Gzip) },
	}
}

// UnmarshalKenginefile sets up the handler from Kenginefile tokens.
func (g *Gzip) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume option name
	if !d.NextArg() {
		return nil
	}
	levelStr := d.Val()
	level, err := strconv.Atoi(levelStr)
	if err != nil {
		return err
	}
	g.Level = level
	return nil
}

// Provision provisions g's configuration.
func (g *Gzip) Provision(ctx kengine.Context) error {
	if g.Level == 0 {
		g.Level = defaultGzipLevel
	}
	return nil
}

// Validate validates g's configuration.
func (g Gzip) Validate() error {
	if g.Level < gzip.StatelessCompression {
		return fmt.Errorf("quality too low; must be >= %d", gzip.StatelessCompression)
	}
	if g.Level > gzip.BestCompression {
		return fmt.Errorf("quality too high; must be <= %d", gzip.BestCompression)
	}
	return nil
}

// AcceptEncoding returns the name of the encoding as
// used in the Accept-Encoding request headers.
func (Gzip) AcceptEncoding() string { return "gzip" }

// NewEncoder returns a new gzip writer.
func (g Gzip) NewEncoder() encode.Encoder {
	writer, _ := gzip.NewWriterLevel(nil, g.Level)
	return writer
}

// Informed from http://blog.klauspost.com/gzip-performance-for-go-webservers/
var defaultGzipLevel = 5

// Interface guards
var (
	_ encode.Encoding         = (*Gzip)(nil)
	_ kengine.Provisioner     = (*Gzip)(nil)
	_ kengine.Validator       = (*Gzip)(nil)
	_ kenginefile.Unmarshaler = (*Gzip)(nil)
)
