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

package reverseproxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	weakrand "math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp"
)

func init() {
	kengine.RegisterModule(RandomSelection{})
	kengine.RegisterModule(RandomChoiceSelection{})
	kengine.RegisterModule(LeastConnSelection{})
	kengine.RegisterModule(RoundRobinSelection{})
	kengine.RegisterModule(WeightedRoundRobinSelection{})
	kengine.RegisterModule(FirstSelection{})
	kengine.RegisterModule(IPHashSelection{})
	kengine.RegisterModule(ClientIPHashSelection{})
	kengine.RegisterModule(URIHashSelection{})
	kengine.RegisterModule(QueryHashSelection{})
	kengine.RegisterModule(HeaderHashSelection{})
	kengine.RegisterModule(CookieHashSelection{})
}

// RandomSelection is a policy that selects
// an available host at random.
type RandomSelection struct{}

// KengineModule returns the Kengine module information.
func (RandomSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.random",
		New: func() kengine.Module { return new(RandomSelection) },
	}
}

// Select returns an available host, if any.
func (r RandomSelection) Select(pool UpstreamPool, request *http.Request, _ http.ResponseWriter) *Upstream {
	return selectRandomHost(pool)
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (r *RandomSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// WeightedRoundRobinSelection is a policy that selects
// a host based on weighted round-robin ordering.
type WeightedRoundRobinSelection struct {
	// The weight of each upstream in order,
	// corresponding with the list of upstreams configured.
	Weights     []int `json:"weights,omitempty"`
	index       uint32
	totalWeight int
}

// KengineModule returns the Kengine module information.
func (WeightedRoundRobinSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID: "http.reverse_proxy.selection_policies.weighted_round_robin",
		New: func() kengine.Module {
			return new(WeightedRoundRobinSelection)
		},
	}
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (r *WeightedRoundRobinSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume policy name

	args := d.RemainingArgs()
	if len(args) == 0 {
		return d.ArgErr()
	}

	for _, weight := range args {
		weightInt, err := strconv.Atoi(weight)
		if err != nil {
			return d.Errf("invalid weight value '%s': %v", weight, err)
		}
		if weightInt < 1 {
			return d.Errf("invalid weight value '%s': weight should be non-zero and positive", weight)
		}
		r.Weights = append(r.Weights, weightInt)
	}
	return nil
}

// Provision sets up r.
func (r *WeightedRoundRobinSelection) Provision(ctx kengine.Context) error {
	for _, weight := range r.Weights {
		r.totalWeight += weight
	}
	return nil
}

// Select returns an available host, if any.
func (r *WeightedRoundRobinSelection) Select(pool UpstreamPool, _ *http.Request, _ http.ResponseWriter) *Upstream {
	if len(pool) == 0 {
		return nil
	}
	if len(r.Weights) < 2 {
		return pool[0]
	}
	var index, totalWeight int
	currentWeight := int(atomic.AddUint32(&r.index, 1)) % r.totalWeight
	for i, weight := range r.Weights {
		totalWeight += weight
		if currentWeight < totalWeight {
			index = i
			break
		}
	}

	upstreams := make([]*Upstream, 0, len(r.Weights))
	for _, upstream := range pool {
		if !upstream.Available() {
			continue
		}
		upstreams = append(upstreams, upstream)
		if len(upstreams) == cap(upstreams) {
			break
		}
	}
	if len(upstreams) == 0 {
		return nil
	}
	return upstreams[index%len(upstreams)]
}

// RandomChoiceSelection is a policy that selects
// two or more available hosts at random, then
// chooses the one with the least load.
type RandomChoiceSelection struct {
	// The size of the sub-pool created from the larger upstream pool. The default value
	// is 2 and the maximum at selection time is the size of the upstream pool.
	Choose int `json:"choose,omitempty"`
}

// KengineModule returns the Kengine module information.
func (RandomChoiceSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.random_choose",
		New: func() kengine.Module { return new(RandomChoiceSelection) },
	}
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (r *RandomChoiceSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume policy name

	if !d.NextArg() {
		return d.ArgErr()
	}
	chooseStr := d.Val()
	choose, err := strconv.Atoi(chooseStr)
	if err != nil {
		return d.Errf("invalid choice value '%s': %v", chooseStr, err)
	}
	r.Choose = choose
	return nil
}

// Provision sets up r.
func (r *RandomChoiceSelection) Provision(ctx kengine.Context) error {
	if r.Choose == 0 {
		r.Choose = 2
	}
	return nil
}

// Validate ensures that r's configuration is valid.
func (r RandomChoiceSelection) Validate() error {
	if r.Choose < 2 {
		return fmt.Errorf("choose must be at least 2")
	}
	return nil
}

// Select returns an available host, if any.
func (r RandomChoiceSelection) Select(pool UpstreamPool, _ *http.Request, _ http.ResponseWriter) *Upstream {
	k := r.Choose
	if k > len(pool) {
		k = len(pool)
	}
	choices := make([]*Upstream, k)
	for i, upstream := range pool {
		if !upstream.Available() {
			continue
		}
		j := weakrand.Intn(i + 1) //nolint:gosec
		if j < k {
			choices[j] = upstream
		}
	}
	return leastRequests(choices)
}

// LeastConnSelection is a policy that selects the
// host with the least active requests. If multiple
// hosts have the same fewest number, one is chosen
// randomly. The term "conn" or "connection" is used
// in this policy name due to its similar meaning in
// other software, but our load balancer actually
// counts active requests rather than connections,
// since these days requests are multiplexed onto
// shared connections.
type LeastConnSelection struct{}

// KengineModule returns the Kengine module information.
func (LeastConnSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.least_conn",
		New: func() kengine.Module { return new(LeastConnSelection) },
	}
}

// Select selects the up host with the least number of connections in the
// pool. If more than one host has the same least number of connections,
// one of the hosts is chosen at random.
func (LeastConnSelection) Select(pool UpstreamPool, _ *http.Request, _ http.ResponseWriter) *Upstream {
	var bestHost *Upstream
	var count int
	leastReqs := -1

	for _, host := range pool {
		if !host.Available() {
			continue
		}
		numReqs := host.NumRequests()
		if leastReqs == -1 || numReqs < leastReqs {
			leastReqs = numReqs
			count = 0
		}

		// among hosts with same least connections, perform a reservoir
		// sample: https://en.wikipedia.org/wiki/Reservoir_sampling
		if numReqs == leastReqs {
			count++
			if count == 1 || (weakrand.Int()%count) == 0 { //nolint:gosec
				bestHost = host
			}
		}
	}

	return bestHost
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (r *LeastConnSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// RoundRobinSelection is a policy that selects
// a host based on round-robin ordering.
type RoundRobinSelection struct {
	robin uint32
}

// KengineModule returns the Kengine module information.
func (RoundRobinSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.round_robin",
		New: func() kengine.Module { return new(RoundRobinSelection) },
	}
}

// Select returns an available host, if any.
func (r *RoundRobinSelection) Select(pool UpstreamPool, _ *http.Request, _ http.ResponseWriter) *Upstream {
	n := uint32(len(pool))
	if n == 0 {
		return nil
	}
	for i := uint32(0); i < n; i++ {
		robin := atomic.AddUint32(&r.robin, 1)
		host := pool[robin%n]
		if host.Available() {
			return host
		}
	}
	return nil
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (r *RoundRobinSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// FirstSelection is a policy that selects
// the first available host.
type FirstSelection struct{}

// KengineModule returns the Kengine module information.
func (FirstSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.first",
		New: func() kengine.Module { return new(FirstSelection) },
	}
}

// Select returns an available host, if any.
func (FirstSelection) Select(pool UpstreamPool, _ *http.Request, _ http.ResponseWriter) *Upstream {
	for _, host := range pool {
		if host.Available() {
			return host
		}
	}
	return nil
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (r *FirstSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// IPHashSelection is a policy that selects a host
// based on hashing the remote IP of the request.
type IPHashSelection struct{}

// KengineModule returns the Kengine module information.
func (IPHashSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.ip_hash",
		New: func() kengine.Module { return new(IPHashSelection) },
	}
}

// Select returns an available host, if any.
func (IPHashSelection) Select(pool UpstreamPool, req *http.Request, _ http.ResponseWriter) *Upstream {
	clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		clientIP = req.RemoteAddr
	}
	return hostByHashing(pool, clientIP)
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (r *IPHashSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// ClientIPHashSelection is a policy that selects a host
// based on hashing the client IP of the request, as determined
// by the HTTP app's trusted proxies settings.
type ClientIPHashSelection struct{}

// KengineModule returns the Kengine module information.
func (ClientIPHashSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.client_ip_hash",
		New: func() kengine.Module { return new(ClientIPHashSelection) },
	}
}

// Select returns an available host, if any.
func (ClientIPHashSelection) Select(pool UpstreamPool, req *http.Request, _ http.ResponseWriter) *Upstream {
	address := kenginehttp.GetVar(req.Context(), kenginehttp.ClientIPVarKey).(string)
	clientIP, _, err := net.SplitHostPort(address)
	if err != nil {
		clientIP = address // no port
	}
	return hostByHashing(pool, clientIP)
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (r *ClientIPHashSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// URIHashSelection is a policy that selects a
// host by hashing the request URI.
type URIHashSelection struct{}

// KengineModule returns the Kengine module information.
func (URIHashSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.uri_hash",
		New: func() kengine.Module { return new(URIHashSelection) },
	}
}

// Select returns an available host, if any.
func (URIHashSelection) Select(pool UpstreamPool, req *http.Request, _ http.ResponseWriter) *Upstream {
	return hostByHashing(pool, req.RequestURI)
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (r *URIHashSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// QueryHashSelection is a policy that selects
// a host based on a given request query parameter.
type QueryHashSelection struct {
	// The query key whose value is to be hashed and used for upstream selection.
	Key string `json:"key,omitempty"`

	// The fallback policy to use if the query key is not present. Defaults to `random`.
	FallbackRaw json.RawMessage `json:"fallback,omitempty" kengine:"namespace=http.reverse_proxy.selection_policies inline_key=policy"`
	fallback    Selector
}

// KengineModule returns the Kengine module information.
func (QueryHashSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.query",
		New: func() kengine.Module { return new(QueryHashSelection) },
	}
}

// Provision sets up the module.
func (s *QueryHashSelection) Provision(ctx kengine.Context) error {
	if s.Key == "" {
		return fmt.Errorf("query key is required")
	}
	if s.FallbackRaw == nil {
		s.FallbackRaw = kengineconfig.JSONModuleObject(RandomSelection{}, "policy", "random", nil)
	}
	mod, err := ctx.LoadModule(s, "FallbackRaw")
	if err != nil {
		return fmt.Errorf("loading fallback selection policy: %s", err)
	}
	s.fallback = mod.(Selector)
	return nil
}

// Select returns an available host, if any.
func (s QueryHashSelection) Select(pool UpstreamPool, req *http.Request, _ http.ResponseWriter) *Upstream {
	// Since the query may have multiple values for the same key,
	// we'll join them to avoid a problem where the user can control
	// the upstream that the request goes to by sending multiple values
	// for the same key, when the upstream only considers the first value.
	// Keep in mind that a client changing the order of the values may
	// affect which upstream is selected, but this is a semantically
	// different request, because the order of the values is significant.
	vals := strings.Join(req.URL.Query()[s.Key], ",")
	if vals == "" {
		return s.fallback.Select(pool, req, nil)
	}
	return hostByHashing(pool, vals)
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (s *QueryHashSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume policy name

	if !d.NextArg() {
		return d.ArgErr()
	}
	s.Key = d.Val()

	for d.NextBlock(0) {
		switch d.Val() {
		case "fallback":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if s.FallbackRaw != nil {
				return d.Err("fallback selection policy already specified")
			}
			mod, err := loadFallbackPolicy(d)
			if err != nil {
				return err
			}
			s.FallbackRaw = mod
		default:
			return d.Errf("unrecognized option '%s'", d.Val())
		}
	}
	return nil
}

// HeaderHashSelection is a policy that selects
// a host based on a given request header.
type HeaderHashSelection struct {
	// The HTTP header field whose value is to be hashed and used for upstream selection.
	Field string `json:"field,omitempty"`

	// The fallback policy to use if the header is not present. Defaults to `random`.
	FallbackRaw json.RawMessage `json:"fallback,omitempty" kengine:"namespace=http.reverse_proxy.selection_policies inline_key=policy"`
	fallback    Selector
}

// KengineModule returns the Kengine module information.
func (HeaderHashSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.header",
		New: func() kengine.Module { return new(HeaderHashSelection) },
	}
}

// Provision sets up the module.
func (s *HeaderHashSelection) Provision(ctx kengine.Context) error {
	if s.Field == "" {
		return fmt.Errorf("header field is required")
	}
	if s.FallbackRaw == nil {
		s.FallbackRaw = kengineconfig.JSONModuleObject(RandomSelection{}, "policy", "random", nil)
	}
	mod, err := ctx.LoadModule(s, "FallbackRaw")
	if err != nil {
		return fmt.Errorf("loading fallback selection policy: %s", err)
	}
	s.fallback = mod.(Selector)
	return nil
}

// Select returns an available host, if any.
func (s HeaderHashSelection) Select(pool UpstreamPool, req *http.Request, _ http.ResponseWriter) *Upstream {
	// The Host header should be obtained from the req.Host field
	// since net/http removes it from the header map.
	if s.Field == "Host" && req.Host != "" {
		return hostByHashing(pool, req.Host)
	}

	val := req.Header.Get(s.Field)
	if val == "" {
		return s.fallback.Select(pool, req, nil)
	}
	return hostByHashing(pool, val)
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens.
func (s *HeaderHashSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	d.Next() // consume policy name

	if !d.NextArg() {
		return d.ArgErr()
	}
	s.Field = d.Val()

	for d.NextBlock(0) {
		switch d.Val() {
		case "fallback":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if s.FallbackRaw != nil {
				return d.Err("fallback selection policy already specified")
			}
			mod, err := loadFallbackPolicy(d)
			if err != nil {
				return err
			}
			s.FallbackRaw = mod
		default:
			return d.Errf("unrecognized option '%s'", d.Val())
		}
	}
	return nil
}

// CookieHashSelection is a policy that selects
// a host based on a given cookie name.
type CookieHashSelection struct {
	// The HTTP cookie name whose value is to be hashed and used for upstream selection.
	Name string `json:"name,omitempty"`
	// Secret to hash (Hmac256) chosen upstream in cookie
	Secret string `json:"secret,omitempty"`
	// The cookie's Max-Age before it expires. Default is no expiry.
	MaxAge kengine.Duration `json:"max_age,omitempty"`

	// The fallback policy to use if the cookie is not present. Defaults to `random`.
	FallbackRaw json.RawMessage `json:"fallback,omitempty" kengine:"namespace=http.reverse_proxy.selection_policies inline_key=policy"`
	fallback    Selector
}

// KengineModule returns the Kengine module information.
func (CookieHashSelection) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.cookie",
		New: func() kengine.Module { return new(CookieHashSelection) },
	}
}

// Provision sets up the module.
func (s *CookieHashSelection) Provision(ctx kengine.Context) error {
	if s.Name == "" {
		s.Name = "lb"
	}
	if s.FallbackRaw == nil {
		s.FallbackRaw = kengineconfig.JSONModuleObject(RandomSelection{}, "policy", "random", nil)
	}
	mod, err := ctx.LoadModule(s, "FallbackRaw")
	if err != nil {
		return fmt.Errorf("loading fallback selection policy: %s", err)
	}
	s.fallback = mod.(Selector)
	return nil
}

// Select returns an available host, if any.
func (s CookieHashSelection) Select(pool UpstreamPool, req *http.Request, w http.ResponseWriter) *Upstream {
	// selects a new Host using the fallback policy (typically random)
	// and write a sticky session cookie to the response.
	selectNewHost := func() *Upstream {
		upstream := s.fallback.Select(pool, req, w)
		if upstream == nil {
			return nil
		}
		sha, err := hashCookie(s.Secret, upstream.Dial)
		if err != nil {
			return upstream
		}
		cookie := &http.Cookie{
			Name:   s.Name,
			Value:  sha,
			Path:   "/",
			Secure: false,
		}
		isProxyHttps := false
		if trusted, ok := kenginehttp.GetVar(req.Context(), kenginehttp.TrustedProxyVarKey).(bool); ok && trusted {
			xfp, xfpOk, _ := lastHeaderValue(req.Header, "X-Forwarded-Proto")
			isProxyHttps = xfpOk && xfp == "https"
		}
		if req.TLS != nil || isProxyHttps {
			cookie.Secure = true
			cookie.SameSite = http.SameSiteNoneMode
		}
		if s.MaxAge > 0 {
			cookie.MaxAge = int(time.Duration(s.MaxAge).Seconds())
		}
		http.SetCookie(w, cookie)
		return upstream
	}

	cookie, err := req.Cookie(s.Name)
	// If there's no cookie, select a host using the fallback policy
	if err != nil || cookie == nil {
		return selectNewHost()
	}
	// If the cookie is present, loop over the available upstreams until we find a match
	cookieValue := cookie.Value
	for _, upstream := range pool {
		if !upstream.Available() {
			continue
		}
		sha, err := hashCookie(s.Secret, upstream.Dial)
		if err == nil && sha == cookieValue {
			return upstream
		}
	}
	// If there is no matching host, select a host using the fallback policy
	return selectNewHost()
}

// UnmarshalKenginefile sets up the module from Kenginefile tokens. Syntax:
//
//	lb_policy cookie [<name> [<secret>]] {
//		fallback <policy>
//		max_age <duration>
//	}
//
// By default name is `lb`
func (s *CookieHashSelection) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	args := d.RemainingArgs()
	switch len(args) {
	case 1:
	case 2:
		s.Name = args[1]
	case 3:
		s.Name = args[1]
		s.Secret = args[2]
	default:
		return d.ArgErr()
	}
	for d.NextBlock(0) {
		switch d.Val() {
		case "fallback":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if s.FallbackRaw != nil {
				return d.Err("fallback selection policy already specified")
			}
			mod, err := loadFallbackPolicy(d)
			if err != nil {
				return err
			}
			s.FallbackRaw = mod
		case "max_age":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if s.MaxAge != 0 {
				return d.Err("cookie max_age already specified")
			}
			maxAge, err := kengine.ParseDuration(d.Val())
			if err != nil {
				return d.Errf("invalid duration: %s", d.Val())
			}
			if maxAge <= 0 {
				return d.Errf("invalid duration: %s, max_age should be non-zero and positive", d.Val())
			}
			if d.NextArg() {
				return d.ArgErr()
			}
			s.MaxAge = kengine.Duration(maxAge)
		default:
			return d.Errf("unrecognized option '%s'", d.Val())
		}
	}
	return nil
}

// hashCookie hashes (HMAC 256) some data with the secret
func hashCookie(secret string, data string) (string, error) {
	h := hmac.New(sha256.New, []byte(secret))
	_, err := h.Write([]byte(data))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// selectRandomHost returns a random available host
func selectRandomHost(pool []*Upstream) *Upstream {
	// use reservoir sampling because the number of available
	// hosts isn't known: https://en.wikipedia.org/wiki/Reservoir_sampling
	var randomHost *Upstream
	var count int
	for _, upstream := range pool {
		if !upstream.Available() {
			continue
		}
		// (n % 1 == 0) holds for all n, therefore a
		// upstream will always be chosen if there is at
		// least one available
		count++
		if (weakrand.Int() % count) == 0 { //nolint:gosec
			randomHost = upstream
		}
	}
	return randomHost
}

// leastRequests returns the host with the
// least number of active requests to it.
// If more than one host has the same
// least number of active requests, then
// one of those is chosen at random.
func leastRequests(upstreams []*Upstream) *Upstream {
	if len(upstreams) == 0 {
		return nil
	}
	var best []*Upstream
	var bestReqs int = -1
	for _, upstream := range upstreams {
		if upstream == nil {
			continue
		}
		reqs := upstream.NumRequests()
		if reqs == 0 {
			return upstream
		}
		// If bestReqs was just initialized to -1
		// we need to append upstream also
		if reqs <= bestReqs || bestReqs == -1 {
			bestReqs = reqs
			best = append(best, upstream)
		}
	}
	if len(best) == 0 {
		return nil
	}
	if len(best) == 1 {
		return best[0]
	}
	return best[weakrand.Intn(len(best))] //nolint:gosec
}

// hostByHashing returns an available host from pool based on a hashable string s.
func hostByHashing(pool []*Upstream, s string) *Upstream {
	// Highest Random Weight (HRW, or "Rendezvous") hashing,
	// guarantees stability when the list of upstreams changes;
	// see https://medium.com/i0exception/rendezvous-hashing-8c00e2fb58b0,
	// https://randorithms.com/2020/12/26/rendezvous-hashing.html,
	// and https://en.wikipedia.org/wiki/Rendezvous_hashing.
	var highestHash uint64
	var upstream *Upstream
	for _, up := range pool {
		if !up.Available() {
			continue
		}
		h := hash(up.String() + s) // important to hash key and server together
		if h > highestHash {
			highestHash = h
			upstream = up
		}
	}
	return upstream
}

// hash calculates a fast hash based on s.
func hash(s string) uint64 {
	h := xxhash.New()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func loadFallbackPolicy(d *kenginefile.Dispenser) (json.RawMessage, error) {
	name := d.Val()
	modID := "http.reverse_proxy.selection_policies." + name
	unm, err := kenginefile.UnmarshalModule(d, modID)
	if err != nil {
		return nil, err
	}
	sel, ok := unm.(Selector)
	if !ok {
		return nil, d.Errf("module %s (%T) is not a reverseproxy.Selector", modID, unm)
	}
	return kengineconfig.JSONModuleObject(sel, "policy", name, nil), nil
}

// Interface guards
var (
	_ Selector = (*RandomSelection)(nil)
	_ Selector = (*RandomChoiceSelection)(nil)
	_ Selector = (*LeastConnSelection)(nil)
	_ Selector = (*RoundRobinSelection)(nil)
	_ Selector = (*WeightedRoundRobinSelection)(nil)
	_ Selector = (*FirstSelection)(nil)
	_ Selector = (*IPHashSelection)(nil)
	_ Selector = (*ClientIPHashSelection)(nil)
	_ Selector = (*URIHashSelection)(nil)
	_ Selector = (*QueryHashSelection)(nil)
	_ Selector = (*HeaderHashSelection)(nil)
	_ Selector = (*CookieHashSelection)(nil)

	_ kengine.Validator = (*RandomChoiceSelection)(nil)

	_ kengine.Provisioner = (*RandomChoiceSelection)(nil)
	_ kengine.Provisioner = (*WeightedRoundRobinSelection)(nil)

	_ kenginefile.Unmarshaler = (*RandomChoiceSelection)(nil)
	_ kenginefile.Unmarshaler = (*WeightedRoundRobinSelection)(nil)
)
