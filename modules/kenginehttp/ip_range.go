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

package kenginehttp

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/internal"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
)

func init() {
	kengine.RegisterModule(StaticIPRange{})
}

// IPRangeSource gets a list of IP ranges.
//
// The request is passed as an argument to allow plugin implementations
// to have more flexibility. But, a plugin MUST NOT modify the request.
// The caller will have read the `r.RemoteAddr` before getting IP ranges.
//
// This should be a very fast function -- instant if possible.
// The list of IP ranges should be sourced as soon as possible if loaded
// from an external source (i.e. initially loaded during Provisioning),
// so that it's ready to be used when requests start getting handled.
// A read lock should probably be used to get the cached value if the
// ranges can change at runtime (e.g. periodically refreshed).
// Using a `kengine.UsagePool` may be a good idea to avoid having refetch
// the values when a config reload occurs, which would waste time.
//
// If the list of IP ranges cannot be sourced, then provisioning SHOULD
// fail. Getting the IP ranges at runtime MUST NOT fail, because it would
// cancel incoming requests. If refreshing the list fails, then the
// previous list of IP ranges should continue to be returned so that the
// server can continue to operate normally.
type IPRangeSource interface {
	GetIPRanges(*http.Request) []netip.Prefix
}

// StaticIPRange provides a static range of IP address prefixes (CIDRs).
type StaticIPRange struct {
	// A static list of IP ranges (supports CIDR notation).
	Ranges []string `json:"ranges,omitempty"`

	// Holds the parsed CIDR ranges from Ranges.
	ranges []netip.Prefix
}

// KengineModule returns the Kengine module information.
func (StaticIPRange) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.ip_sources.static",
		New: func() kengine.Module { return new(StaticIPRange) },
	}
}

func (s *StaticIPRange) Provision(ctx kengine.Context) error {
	for _, str := range s.Ranges {
		prefix, err := CIDRExpressionToPrefix(str)
		if err != nil {
			return err
		}
		s.ranges = append(s.ranges, prefix)
	}

	return nil
}

func (s *StaticIPRange) GetIPRanges(_ *http.Request) []netip.Prefix {
	return s.ranges
}

// UnmarshalKenginefile implements kenginefile.Unmarshaler.
func (m *StaticIPRange) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	if !d.Next() {
		return nil
	}
	for d.NextArg() {
		if d.Val() == "private_ranges" {
			m.Ranges = append(m.Ranges, internal.PrivateRangesCIDR()...)
			continue
		}
		m.Ranges = append(m.Ranges, d.Val())
	}
	return nil
}

// CIDRExpressionToPrefix takes a string which could be either a
// CIDR expression or a single IP address, and returns a netip.Prefix.
func CIDRExpressionToPrefix(expr string) (netip.Prefix, error) {
	// Having a slash means it should be a CIDR expression
	if strings.Contains(expr, "/") {
		prefix, err := netip.ParsePrefix(expr)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("parsing CIDR expression: '%s': %v", expr, err)
		}
		return prefix, nil
	}

	// Otherwise it's likely a single IP address
	parsed, err := netip.ParseAddr(expr)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid IP address: '%s': %v", expr, err)
	}
	prefix := netip.PrefixFrom(parsed, parsed.BitLen())
	return prefix, nil
}

// Interface guards
var (
	_ kengine.Provisioner     = (*StaticIPRange)(nil)
	_ kenginefile.Unmarshaler = (*StaticIPRange)(nil)
	_ IPRangeSource           = (*StaticIPRange)(nil)
)

// PrivateRangesCIDR returns a list of private CIDR range
// strings, which can be used as a configuration shortcut.
// Note: this function is used at least by mholt/kengine-l4.
func PrivateRangesCIDR() []string {
	return internal.PrivateRangesCIDR()
}
