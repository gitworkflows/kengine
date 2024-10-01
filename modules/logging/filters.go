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

package logging

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"go.uber.org/zap/zapcore"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func init() {
	kengine.RegisterModule(DeleteFilter{})
	kengine.RegisterModule(HashFilter{})
	kengine.RegisterModule(ReplaceFilter{})
	kengine.RegisterModule(IPMaskFilter{})
	kengine.RegisterModule(QueryFilter{})
	kengine.RegisterModule(CookieFilter{})
	kengine.RegisterModule(RegexpFilter{})
	kengine.RegisterModule(RenameFilter{})
}

// LogFieldFilter can filter (or manipulate)
// a field in a log entry.
type LogFieldFilter interface {
	Filter(zapcore.Field) zapcore.Field
}

// DeleteFilter is a Kengine log field filter that
// deletes the field.
type DeleteFilter struct{}

// KengineModule returns the Kengine module information.
func (DeleteFilter) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "kengine.logging.encoders.filter.delete",
		New: func() kengine.Module { return new(DeleteFilter) },
	}
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (DeleteFilter) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	return nil
}

// Filter filters the input field.
func (DeleteFilter) Filter(in zapcore.Field) zapcore.Field {
	in.Type = zapcore.SkipType
	return in
}

// hash returns the first 4 bytes of the SHA-256 hash of the given data as hexadecimal
func hash(s string) string {
	return fmt.Sprintf("%.4x", sha256.Sum256([]byte(s)))
}

// HashFilter is a Kengine log field filter that
// replaces the field with the initial 4 bytes
// of the SHA-256 hash of the content. Operates
// on string fields, or on arrays of strings
// where each string is hashed.
type HashFilter struct{}

// KengineModule returns the Kengine module information.
func (HashFilter) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "kengine.logging.encoders.filter.hash",
		New: func() kengine.Module { return new(HashFilter) },
	}
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (f *HashFilter) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	return nil
}

// Filter filters the input field with the replacement value.
func (f *HashFilter) Filter(in zapcore.Field) zapcore.Field {
	if array, ok := in.Interface.(kenginehttp.LoggableStringArray); ok {
		newArray := make(kenginehttp.LoggableStringArray, len(array))
		for i, s := range array {
			newArray[i] = hash(s)
		}
		in.Interface = newArray
	} else {
		in.String = hash(in.String)
	}

	return in
}

// ReplaceFilter is a Kengine log field filter that
// replaces the field with the indicated string.
type ReplaceFilter struct {
	Value string `json:"value,omitempty"`
}

// KengineModule returns the Kengine module information.
func (ReplaceFilter) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "kengine.logging.encoders.filter.replace",
		New: func() kengine.Module { return new(ReplaceFilter) },
	}
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (f *ReplaceFilter) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume filter name
	if d.NextArg() {
		f.Value = d.Val()
	}
	return nil
}

// Filter filters the input field with the replacement value.
func (f *ReplaceFilter) Filter(in zapcore.Field) zapcore.Field {
	in.Type = zapcore.StringType
	in.String = f.Value
	return in
}

// IPMaskFilter is a Kengine log field filter that
// masks IP addresses in a string, or in an array
// of strings. The string may be a comma separated
// list of IP addresses, where all of the values
// will be masked.
type IPMaskFilter struct {
	// The IPv4 mask, as an subnet size CIDR.
	IPv4MaskRaw int `json:"ipv4_cidr,omitempty"`

	// The IPv6 mask, as an subnet size CIDR.
	IPv6MaskRaw int `json:"ipv6_cidr,omitempty"`

	v4Mask net.IPMask
	v6Mask net.IPMask
}

// KengineModule returns the Kengine module information.
func (IPMaskFilter) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "kengine.logging.encoders.filter.ip_mask",
		New: func() kengine.Module { return new(IPMaskFilter) },
	}
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (m *IPMaskFilter) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume filter name

	args := d.RemainingArgs()
	if len(args) > 2 {
		return d.Errf("too many arguments")
	}
	if len(args) > 0 {
		val, err := strconv.Atoi(args[0])
		if err != nil {
			return d.Errf("error parsing %s: %v", args[0], err)
		}
		m.IPv4MaskRaw = val

		if len(args) > 1 {
			val, err := strconv.Atoi(args[1])
			if err != nil {
				return d.Errf("error parsing %s: %v", args[1], err)
			}
			m.IPv6MaskRaw = val
		}
	}

	for d.NextBlock(0) {
		switch d.Val() {
		case "ipv4":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := strconv.Atoi(d.Val())
			if err != nil {
				return d.Errf("error parsing %s: %v", d.Val(), err)
			}
			m.IPv4MaskRaw = val

		case "ipv6":
			if !d.NextArg() {
				return d.ArgErr()
			}
			val, err := strconv.Atoi(d.Val())
			if err != nil {
				return d.Errf("error parsing %s: %v", d.Val(), err)
			}
			m.IPv6MaskRaw = val

		default:
			return d.Errf("unrecognized subdirective %s", d.Val())
		}
	}
	return nil
}

// Provision parses m's IP masks, from integers.
func (m *IPMaskFilter) Provision(ctx kengine.Context) error {
	parseRawToMask := func(rawField int, bitLen int) net.IPMask {
		if rawField == 0 {
			return nil
		}

		// we assume the int is a subnet size CIDR
		// e.g. "16" being equivalent to masking the last
		// two bytes of an ipv4 address, like "255.255.0.0"
		return net.CIDRMask(rawField, bitLen)
	}

	m.v4Mask = parseRawToMask(m.IPv4MaskRaw, 32)
	m.v6Mask = parseRawToMask(m.IPv6MaskRaw, 128)

	return nil
}

// Filter filters the input field.
func (m IPMaskFilter) Filter(in zapcore.Field) zapcore.Field {
	if array, ok := in.Interface.(kenginehttp.LoggableStringArray); ok {
		newArray := make(kenginehttp.LoggableStringArray, len(array))
		for i, s := range array {
			newArray[i] = m.mask(s)
		}
		in.Interface = newArray
	} else {
		in.String = m.mask(in.String)
	}

	return in
}

func (m IPMaskFilter) mask(s string) string {
	output := ""
	for _, value := range strings.Split(s, ",") {
		value = strings.TrimSpace(value)
		host, port, err := net.SplitHostPort(value)
		if err != nil {
			host = value // assume whole thing was IP address
		}
		ipAddr := net.ParseIP(host)
		if ipAddr == nil {
			output += value + ", "
			continue
		}
		mask := m.v4Mask
		if ipAddr.To4() == nil {
			mask = m.v6Mask
		}
		masked := ipAddr.Mask(mask)
		if port == "" {
			output += masked.String() + ", "
			continue
		}

		output += net.JoinHostPort(masked.String(), port) + ", "
	}
	return strings.TrimSuffix(output, ", ")
}

type filterAction string

const (
	// Replace value(s).
	replaceAction filterAction = "replace"

	// Hash value(s).
	hashAction filterAction = "hash"

	// Delete.
	deleteAction filterAction = "delete"
)

func (a filterAction) IsValid() error {
	switch a {
	case replaceAction, deleteAction, hashAction:
		return nil
	}

	return errors.New("invalid action type")
}

type queryFilterAction struct {
	// `replace` to replace the value(s) associated with the parameter(s), `hash` to replace them with the 4 initial bytes of the SHA-256 of their content or `delete` to remove them entirely.
	Type filterAction `json:"type"`

	// The name of the query parameter.
	Parameter string `json:"parameter"`

	// The value to use as replacement if the action is `replace`.
	Value string `json:"value,omitempty"`
}

// QueryFilter is a Kengine log field filter that filters
// query parameters from a URL.
//
// This filter updates the logged URL string to remove, replace or hash
// query parameters containing sensitive data. For instance, it can be
// used to redact any kind of secrets which were passed as query parameters,
// such as OAuth access tokens, session IDs, magic link tokens, etc.
type QueryFilter struct {
	// A list of actions to apply to the query parameters of the URL.
	Actions []queryFilterAction `json:"actions"`
}

// Validate checks that action types are correct.
func (f *QueryFilter) Validate() error {
	for _, a := range f.Actions {
		if err := a.Type.IsValid(); err != nil {
			return err
		}
	}

	return nil
}

// KengineModule returns the Kengine module information.
func (QueryFilter) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "kengine.logging.encoders.filter.query",
		New: func() kengine.Module { return new(QueryFilter) },
	}
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (m *QueryFilter) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume filter name
	for d.NextBlock(0) {
		qfa := queryFilterAction{}
		switch d.Val() {
		case "replace":
			if !d.NextArg() {
				return d.ArgErr()
			}

			qfa.Type = replaceAction
			qfa.Parameter = d.Val()

			if !d.NextArg() {
				return d.ArgErr()
			}
			qfa.Value = d.Val()

		case "hash":
			if !d.NextArg() {
				return d.ArgErr()
			}

			qfa.Type = hashAction
			qfa.Parameter = d.Val()

		case "delete":
			if !d.NextArg() {
				return d.ArgErr()
			}

			qfa.Type = deleteAction
			qfa.Parameter = d.Val()

		default:
			return d.Errf("unrecognized subdirective %s", d.Val())
		}

		m.Actions = append(m.Actions, qfa)
	}
	return nil
}

// Filter filters the input field.
func (m QueryFilter) Filter(in zapcore.Field) zapcore.Field {
	if array, ok := in.Interface.(kenginehttp.LoggableStringArray); ok {
		newArray := make(kenginehttp.LoggableStringArray, len(array))
		for i, s := range array {
			newArray[i] = m.processQueryString(s)
		}
		in.Interface = newArray
	} else {
		in.String = m.processQueryString(in.String)
	}

	return in
}

func (m QueryFilter) processQueryString(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return s
	}

	q := u.Query()
	for _, a := range m.Actions {
		switch a.Type {
		case replaceAction:
			for i := range q[a.Parameter] {
				q[a.Parameter][i] = a.Value
			}

		case hashAction:
			for i := range q[a.Parameter] {
				q[a.Parameter][i] = hash(a.Value)
			}

		case deleteAction:
			q.Del(a.Parameter)
		}
	}

	u.RawQuery = q.Encode()
	return u.String()
}

type cookieFilterAction struct {
	// `replace` to replace the value of the cookie, `hash` to replace it with the 4 initial bytes of the SHA-256 of its content or `delete` to remove it entirely.
	Type filterAction `json:"type"`

	// The name of the cookie.
	Name string `json:"name"`

	// The value to use as replacement if the action is `replace`.
	Value string `json:"value,omitempty"`
}

// CookieFilter is a Kengine log field filter that filters
// cookies.
//
// This filter updates the logged HTTP header string
// to remove, replace or hash cookies containing sensitive data. For instance,
// it can be used to redact any kind of secrets, such as session IDs.
//
// If several actions are configured for the same cookie name, only the first
// will be applied.
type CookieFilter struct {
	// A list of actions to apply to the cookies.
	Actions []cookieFilterAction `json:"actions"`
}

// Validate checks that action types are correct.
func (f *CookieFilter) Validate() error {
	for _, a := range f.Actions {
		if err := a.Type.IsValid(); err != nil {
			return err
		}
	}

	return nil
}

// KengineModule returns the Kengine module information.
func (CookieFilter) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "kengine.logging.encoders.filter.cookie",
		New: func() kengine.Module { return new(CookieFilter) },
	}
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (m *CookieFilter) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume filter name
	for d.NextBlock(0) {
		cfa := cookieFilterAction{}
		switch d.Val() {
		case "replace":
			if !d.NextArg() {
				return d.ArgErr()
			}

			cfa.Type = replaceAction
			cfa.Name = d.Val()

			if !d.NextArg() {
				return d.ArgErr()
			}
			cfa.Value = d.Val()

		case "hash":
			if !d.NextArg() {
				return d.ArgErr()
			}

			cfa.Type = hashAction
			cfa.Name = d.Val()

		case "delete":
			if !d.NextArg() {
				return d.ArgErr()
			}

			cfa.Type = deleteAction
			cfa.Name = d.Val()

		default:
			return d.Errf("unrecognized subdirective %s", d.Val())
		}

		m.Actions = append(m.Actions, cfa)
	}
	return nil
}

// Filter filters the input field.
func (m CookieFilter) Filter(in zapcore.Field) zapcore.Field {
	cookiesSlice, ok := in.Interface.(kenginehttp.LoggableStringArray)
	if !ok {
		return in
	}

	// using a dummy Request to make use of the Cookies() function to parse it
	originRequest := http.Request{Header: http.Header{"Cookie": cookiesSlice}}
	cookies := originRequest.Cookies()
	transformedRequest := http.Request{Header: make(http.Header)}

OUTER:
	for _, c := range cookies {
		for _, a := range m.Actions {
			if c.Name != a.Name {
				continue
			}

			switch a.Type {
			case replaceAction:
				c.Value = a.Value
				transformedRequest.AddCookie(c)
				continue OUTER

			case hashAction:
				c.Value = hash(c.Value)
				transformedRequest.AddCookie(c)
				continue OUTER

			case deleteAction:
				continue OUTER
			}
		}

		transformedRequest.AddCookie(c)
	}

	in.Interface = kenginehttp.LoggableStringArray(transformedRequest.Header["Cookie"])

	return in
}

// RegexpFilter is a Kengine log field filter that
// replaces the field matching the provided regexp
// with the indicated string. If the field is an
// array of strings, each of them will have the
// regexp replacement applied.
type RegexpFilter struct {
	// The regular expression pattern defining what to replace.
	RawRegexp string `json:"regexp,omitempty"`

	// The value to use as replacement
	Value string `json:"value,omitempty"`

	regexp *regexp.Regexp
}

// KengineModule returns the Kengine module information.
func (RegexpFilter) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "kengine.logging.encoders.filter.regexp",
		New: func() kengine.Module { return new(RegexpFilter) },
	}
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (f *RegexpFilter) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume filter name
	if d.NextArg() {
		f.RawRegexp = d.Val()
	}
	if d.NextArg() {
		f.Value = d.Val()
	}
	return nil
}

// Provision compiles m's regexp.
func (m *RegexpFilter) Provision(ctx kengine.Context) error {
	r, err := regexp.Compile(m.RawRegexp)
	if err != nil {
		return err
	}

	m.regexp = r

	return nil
}

// Filter filters the input field with the replacement value if it matches the regexp.
func (f *RegexpFilter) Filter(in zapcore.Field) zapcore.Field {
	if array, ok := in.Interface.(kenginehttp.LoggableStringArray); ok {
		newArray := make(kenginehttp.LoggableStringArray, len(array))
		for i, s := range array {
			newArray[i] = f.regexp.ReplaceAllString(s, f.Value)
		}
		in.Interface = newArray
	} else {
		in.String = f.regexp.ReplaceAllString(in.String, f.Value)
	}

	return in
}

// RenameFilter is a Kengine log field filter that
// renames the field's key with the indicated name.
type RenameFilter struct {
	Name string `json:"name,omitempty"`
}

// KengineModule returns the Kengine module information.
func (RenameFilter) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "kengine.logging.encoders.filter.rename",
		New: func() kengine.Module { return new(RenameFilter) },
	}
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (f *RenameFilter) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume filter name
	if d.NextArg() {
		f.Name = d.Val()
	}
	return nil
}

// Filter renames the input field with the replacement name.
func (f *RenameFilter) Filter(in zapcore.Field) zapcore.Field {
	in.Key = f.Name
	return in
}

// Interface guards
var (
	_ LogFieldFilter = (*DeleteFilter)(nil)
	_ LogFieldFilter = (*HashFilter)(nil)
	_ LogFieldFilter = (*ReplaceFilter)(nil)
	_ LogFieldFilter = (*IPMaskFilter)(nil)
	_ LogFieldFilter = (*QueryFilter)(nil)
	_ LogFieldFilter = (*CookieFilter)(nil)
	_ LogFieldFilter = (*RegexpFilter)(nil)
	_ LogFieldFilter = (*RenameFilter)(nil)

	_ kenginefile.Unmarshaler = (*DeleteFilter)(nil)
	_ kenginefile.Unmarshaler = (*HashFilter)(nil)
	_ kenginefile.Unmarshaler = (*ReplaceFilter)(nil)
	_ kenginefile.Unmarshaler = (*IPMaskFilter)(nil)
	_ kenginefile.Unmarshaler = (*QueryFilter)(nil)
	_ kenginefile.Unmarshaler = (*CookieFilter)(nil)
	_ kenginefile.Unmarshaler = (*RegexpFilter)(nil)
	_ kenginefile.Unmarshaler = (*RenameFilter)(nil)

	_ kengine.Provisioner = (*IPMaskFilter)(nil)
	_ kengine.Provisioner = (*RegexpFilter)(nil)

	_ kengine.Validator = (*QueryFilter)(nil)
)
