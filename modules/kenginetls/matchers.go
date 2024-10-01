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

package kenginetls

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/khulnasoft-lab/certmagic"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/internal"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
)

func init() {
	kengine.RegisterModule(MatchServerName{})
	kengine.RegisterModule(MatchServerNameRE{})
	kengine.RegisterModule(MatchRemoteIP{})
	kengine.RegisterModule(MatchLocalIP{})
}

// MatchServerName matches based on SNI. Names in
// this list may use left-most-label wildcards,
// similar to wildcard certificates.
type MatchServerName []string

// KengineModule returns the Kengine module information.
func (MatchServerName) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "tls.handshake_match.sni",
		New: func() kengine.Module { return new(MatchServerName) },
	}
}

// Match matches hello based on SNI.
func (m MatchServerName) Match(hello *tls.ClientHelloInfo) bool {
	repl := kengine.NewReplacer()
	// kenginetls.TestServerNameMatcher calls this function without any context
	if ctx := hello.Context(); ctx != nil {
		// In some situations the existing context may have no replacer
		if replAny := ctx.Value(kengine.ReplacerCtxKey); replAny != nil {
			repl = replAny.(*kengine.Replacer)
		}
	}

	for _, name := range m {
		rs := repl.ReplaceAll(name, "")
		if certmagic.MatchWildcard(hello.ServerName, rs) {
			return true
		}
	}
	return false
}

// UnmarshalKenginefile sets up the MatchServerName from Kenginefile tokens. Syntax:
//
//	sni <domains...>
func (m *MatchServerName) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	for d.Next() {
		wrapper := d.Val()

		// At least one same-line option must be provided
		if d.CountRemainingArgs() == 0 {
			return d.ArgErr()
		}

		*m = append(*m, d.RemainingArgs()...)

		// No blocks are supported
		if d.NextBlock(d.Nesting()) {
			return d.Errf("malformed TLS handshake matcher '%s': blocks are not supported", wrapper)
		}
	}

	return nil
}

// MatchRegexp is an embeddable type for matching
// using regular expressions. It adds placeholders
// to the request's replacer. In fact, it is a copy of
// kenginehttp.MatchRegexp with a local replacer prefix
// and placeholders support in a regular expression pattern.
type MatchRegexp struct {
	// A unique name for this regular expression. Optional,
	// but useful to prevent overwriting captures from other
	// regexp matchers.
	Name string `json:"name,omitempty"`

	// The regular expression to evaluate, in RE2 syntax,
	// which is the same general syntax used by Go, Perl,
	// and Python. For details, see
	// [Go's regexp package](https://golang.org/pkg/regexp/).
	// Captures are accessible via placeholders. Unnamed
	// capture groups are exposed as their numeric, 1-based
	// index, while named capture groups are available by
	// the capture group name.
	Pattern string `json:"pattern"`

	compiled *regexp.Regexp
}

// Provision compiles the regular expression which may include placeholders.
func (mre *MatchRegexp) Provision(kengine.Context) error {
	repl := kengine.NewReplacer()
	re, err := regexp.Compile(repl.ReplaceAll(mre.Pattern, ""))
	if err != nil {
		return fmt.Errorf("compiling matcher regexp %s: %v", mre.Pattern, err)
	}
	mre.compiled = re
	return nil
}

// Validate ensures mre is set up correctly.
func (mre *MatchRegexp) Validate() error {
	if mre.Name != "" && !wordRE.MatchString(mre.Name) {
		return fmt.Errorf("invalid regexp name (must contain only word characters): %s", mre.Name)
	}
	return nil
}

// Match returns true if input matches the compiled regular
// expression in m. It sets values on the replacer repl
// associated with capture groups, using the given scope
// (namespace).
func (mre *MatchRegexp) Match(input string, repl *kengine.Replacer) bool {
	matches := mre.compiled.FindStringSubmatch(input)
	if matches == nil {
		return false
	}

	// save all capture groups, first by index
	for i, match := range matches {
		keySuffix := "." + strconv.Itoa(i)
		if mre.Name != "" {
			repl.Set(regexpPlaceholderPrefix+"."+mre.Name+keySuffix, match)
		}
		repl.Set(regexpPlaceholderPrefix+keySuffix, match)
	}

	// then by name
	for i, name := range mre.compiled.SubexpNames() {
		// skip the first element (the full match), and empty names
		if i == 0 || name == "" {
			continue
		}

		keySuffix := "." + name
		if mre.Name != "" {
			repl.Set(regexpPlaceholderPrefix+"."+mre.Name+keySuffix, matches[i])
		}
		repl.Set(regexpPlaceholderPrefix+keySuffix, matches[i])
	}

	return true
}

// UnmarshalKenginefile implements kenginefile.Unmarshaler.
func (mre *MatchRegexp) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	// iterate to merge multiple matchers into one
	for d.Next() {
		// If this is the second iteration of the loop
		// then there's more than one *_regexp matcher,
		// and we would end up overwriting the old one
		if mre.Pattern != "" {
			return d.Err("regular expression can only be used once per named matcher")
		}

		args := d.RemainingArgs()
		switch len(args) {
		case 1:
			mre.Pattern = args[0]
		case 2:
			mre.Name = args[0]
			mre.Pattern = args[1]
		default:
			return d.ArgErr()
		}

		// Default to the named matcher's name, if no regexp name is provided.
		// Note: it requires d.SetContext(kenginefile.MatcherNameCtxKey, value)
		// called before this unmarshalling, otherwise it wouldn't work.
		if mre.Name == "" {
			mre.Name = d.GetContextString(kenginefile.MatcherNameCtxKey)
		}

		if d.NextBlock(0) {
			return d.Err("malformed regexp matcher: blocks are not supported")
		}
	}
	return nil
}

// MatchServerNameRE matches based on SNI using a regular expression.
type MatchServerNameRE struct{ MatchRegexp }

// KengineModule returns the Kengine module information.
func (MatchServerNameRE) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "tls.handshake_match.sni_regexp",
		New: func() kengine.Module { return new(MatchServerNameRE) },
	}
}

// Match matches hello based on SNI using a regular expression.
func (m MatchServerNameRE) Match(hello *tls.ClientHelloInfo) bool {
	repl := kengine.NewReplacer()
	// kenginetls.TestServerNameMatcher calls this function without any context
	if ctx := hello.Context(); ctx != nil {
		// In some situations the existing context may have no replacer
		if replAny := ctx.Value(kengine.ReplacerCtxKey); replAny != nil {
			repl = replAny.(*kengine.Replacer)
		}
	}

	return m.MatchRegexp.Match(hello.ServerName, repl)
}

// MatchRemoteIP matches based on the remote IP of the
// connection. Specific IPs or CIDR ranges can be specified.
//
// Note that IPs can sometimes be spoofed, so do not rely
// on this as a replacement for actual authentication.
type MatchRemoteIP struct {
	// The IPs or CIDR ranges to match.
	Ranges []string `json:"ranges,omitempty"`

	// The IPs or CIDR ranges to *NOT* match.
	NotRanges []string `json:"not_ranges,omitempty"`

	cidrs    []netip.Prefix
	notCidrs []netip.Prefix
	logger   *zap.Logger
}

// KengineModule returns the Kengine module information.
func (MatchRemoteIP) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "tls.handshake_match.remote_ip",
		New: func() kengine.Module { return new(MatchRemoteIP) },
	}
}

// Provision parses m's IP ranges, either from IP or CIDR expressions.
func (m *MatchRemoteIP) Provision(ctx kengine.Context) error {
	repl := kengine.NewReplacer()
	m.logger = ctx.Logger()
	for _, str := range m.Ranges {
		rs := repl.ReplaceAll(str, "")
		cidrs, err := m.parseIPRange(rs)
		if err != nil {
			return err
		}
		m.cidrs = append(m.cidrs, cidrs...)
	}
	for _, str := range m.NotRanges {
		rs := repl.ReplaceAll(str, "")
		cidrs, err := m.parseIPRange(rs)
		if err != nil {
			return err
		}
		m.notCidrs = append(m.notCidrs, cidrs...)
	}
	return nil
}

// Match matches hello based on the connection's remote IP.
func (m MatchRemoteIP) Match(hello *tls.ClientHelloInfo) bool {
	remoteAddr := hello.Conn.RemoteAddr().String()
	ipStr, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		ipStr = remoteAddr // weird; maybe no port?
	}
	ipAddr, err := netip.ParseAddr(ipStr)
	if err != nil {
		if c := m.logger.Check(zapcore.ErrorLevel, "invalid client IP address"); c != nil {
			c.Write(zap.String("ip", ipStr))
		}
		return false
	}
	return (len(m.cidrs) == 0 || m.matches(ipAddr, m.cidrs)) &&
		(len(m.notCidrs) == 0 || !m.matches(ipAddr, m.notCidrs))
}

func (MatchRemoteIP) parseIPRange(str string) ([]netip.Prefix, error) {
	var cidrs []netip.Prefix
	if strings.Contains(str, "/") {
		ipNet, err := netip.ParsePrefix(str)
		if err != nil {
			return nil, fmt.Errorf("parsing CIDR expression: %v", err)
		}
		cidrs = append(cidrs, ipNet)
	} else {
		ipAddr, err := netip.ParseAddr(str)
		if err != nil {
			return nil, fmt.Errorf("invalid IP address: '%s': %v", str, err)
		}
		ip := netip.PrefixFrom(ipAddr, ipAddr.BitLen())
		cidrs = append(cidrs, ip)
	}
	return cidrs, nil
}

func (MatchRemoteIP) matches(ip netip.Addr, ranges []netip.Prefix) bool {
	return slices.ContainsFunc(ranges, func(prefix netip.Prefix) bool {
		return prefix.Contains(ip)
	})
}

// UnmarshalKenginefile sets up the MatchRemoteIP from Kenginefile tokens. Syntax:
//
//	remote_ip <ranges...>
//
// Note: IPs and CIDRs prefixed with ! symbol are treated as not_ranges
func (m *MatchRemoteIP) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	for d.Next() {
		wrapper := d.Val()

		// At least one same-line option must be provided
		if d.CountRemainingArgs() == 0 {
			return d.ArgErr()
		}

		for d.NextArg() {
			val := d.Val()
			var exclamation bool
			if len(val) > 1 && val[0] == '!' {
				exclamation, val = true, val[1:]
			}
			ranges := []string{val}
			if val == "private_ranges" {
				ranges = internal.PrivateRangesCIDR()
			}
			if exclamation {
				m.NotRanges = append(m.NotRanges, ranges...)
			} else {
				m.Ranges = append(m.Ranges, ranges...)
			}
		}

		// No blocks are supported
		if d.NextBlock(d.Nesting()) {
			return d.Errf("malformed TLS handshake matcher '%s': blocks are not supported", wrapper)
		}
	}

	return nil
}

// MatchLocalIP matches based on the IP address of the interface
// receiving the connection. Specific IPs or CIDR ranges can be specified.
type MatchLocalIP struct {
	// The IPs or CIDR ranges to match.
	Ranges []string `json:"ranges,omitempty"`

	cidrs  []netip.Prefix
	logger *zap.Logger
}

// KengineModule returns the Kengine module information.
func (MatchLocalIP) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "tls.handshake_match.local_ip",
		New: func() kengine.Module { return new(MatchLocalIP) },
	}
}

// Provision parses m's IP ranges, either from IP or CIDR expressions.
func (m *MatchLocalIP) Provision(ctx kengine.Context) error {
	repl := kengine.NewReplacer()
	m.logger = ctx.Logger()
	for _, str := range m.Ranges {
		rs := repl.ReplaceAll(str, "")
		cidrs, err := m.parseIPRange(rs)
		if err != nil {
			return err
		}
		m.cidrs = append(m.cidrs, cidrs...)
	}
	return nil
}

// Match matches hello based on the connection's remote IP.
func (m MatchLocalIP) Match(hello *tls.ClientHelloInfo) bool {
	localAddr := hello.Conn.LocalAddr().String()
	ipStr, _, err := net.SplitHostPort(localAddr)
	if err != nil {
		ipStr = localAddr // weird; maybe no port?
	}
	ipAddr, err := netip.ParseAddr(ipStr)
	if err != nil {
		if c := m.logger.Check(zapcore.ErrorLevel, "invalid local IP address"); c != nil {
			c.Write(zap.String("ip", ipStr))
		}
		return false
	}
	return (len(m.cidrs) == 0 || m.matches(ipAddr, m.cidrs))
}

func (MatchLocalIP) parseIPRange(str string) ([]netip.Prefix, error) {
	var cidrs []netip.Prefix
	if strings.Contains(str, "/") {
		ipNet, err := netip.ParsePrefix(str)
		if err != nil {
			return nil, fmt.Errorf("parsing CIDR expression: %v", err)
		}
		cidrs = append(cidrs, ipNet)
	} else {
		ipAddr, err := netip.ParseAddr(str)
		if err != nil {
			return nil, fmt.Errorf("invalid IP address: '%s': %v", str, err)
		}
		ip := netip.PrefixFrom(ipAddr, ipAddr.BitLen())
		cidrs = append(cidrs, ip)
	}
	return cidrs, nil
}

func (MatchLocalIP) matches(ip netip.Addr, ranges []netip.Prefix) bool {
	return slices.ContainsFunc(ranges, func(prefix netip.Prefix) bool {
		return prefix.Contains(ip)
	})
}

// UnmarshalKenginefile sets up the MatchLocalIP from Kenginefile tokens. Syntax:
//
//	local_ip <ranges...>
func (m *MatchLocalIP) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	for d.Next() {
		wrapper := d.Val()

		// At least one same-line option must be provided
		if d.CountRemainingArgs() == 0 {
			return d.ArgErr()
		}

		for d.NextArg() {
			val := d.Val()
			if val == "private_ranges" {
				m.Ranges = append(m.Ranges, internal.PrivateRangesCIDR()...)
				continue
			}
			m.Ranges = append(m.Ranges, val)
		}

		// No blocks are supported
		if d.NextBlock(d.Nesting()) {
			return d.Errf("malformed TLS handshake matcher '%s': blocks are not supported", wrapper)
		}
	}

	return nil
}

// Interface guards
var (
	_ ConnectionMatcher = (*MatchLocalIP)(nil)
	_ ConnectionMatcher = (*MatchRemoteIP)(nil)
	_ ConnectionMatcher = (*MatchServerName)(nil)
	_ ConnectionMatcher = (*MatchServerNameRE)(nil)

	_ kengine.Provisioner = (*MatchLocalIP)(nil)
	_ kengine.Provisioner = (*MatchRemoteIP)(nil)
	_ kengine.Provisioner = (*MatchServerNameRE)(nil)

	_ kenginefile.Unmarshaler = (*MatchLocalIP)(nil)
	_ kenginefile.Unmarshaler = (*MatchRemoteIP)(nil)
	_ kenginefile.Unmarshaler = (*MatchServerName)(nil)
	_ kenginefile.Unmarshaler = (*MatchServerNameRE)(nil)
)

var wordRE = regexp.MustCompile(`\w+`)

const regexpPlaceholderPrefix = "tls.regexp"
