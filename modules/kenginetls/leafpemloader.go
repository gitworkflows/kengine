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
	"crypto/x509"
	"fmt"

	"github.com/khulnasoft/kengine/v2"
)

func init() {
	kengine.RegisterModule(LeafPEMLoader{})
}

// LeafPEMLoader loads leaf certificates by
// decoding their PEM blocks directly. This has the advantage
// of not needing to store them on disk at all.
type LeafPEMLoader struct {
	Certificates []string `json:"certificates,omitempty"`
}

// Provision implements kengine.Provisioner.
func (pl *LeafPEMLoader) Provision(ctx kengine.Context) error {
	repl, ok := ctx.Value(kengine.ReplacerCtxKey).(*kengine.Replacer)
	if !ok {
		repl = kengine.NewReplacer()
	}
	for i, cert := range pl.Certificates {
		pl.Certificates[i] = repl.ReplaceKnown(cert, "")
	}
	return nil
}

// KengineModule returns the Kengine module information.
func (LeafPEMLoader) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "tls.leaf_cert_loader.pem",
		New: func() kengine.Module { return new(LeafPEMLoader) },
	}
}

// LoadLeafCertificates returns the certificates contained in pl.
func (pl LeafPEMLoader) LoadLeafCertificates() ([]*x509.Certificate, error) {
	certs := make([]*x509.Certificate, 0, len(pl.Certificates))
	for i, cert := range pl.Certificates {
		derBytes, err := convertPEMToDER([]byte(cert))
		if err != nil {
			return nil, fmt.Errorf("PEM leaf certificate loader, cert %d: %v", i, err)
		}
		cert, err := x509.ParseCertificate(derBytes)
		if err != nil {
			return nil, fmt.Errorf("PEM cert %d: %v", i, err)
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

// Interface guard
var (
	_ LeafCertificateLoader = (*LeafPEMLoader)(nil)
	_ kengine.Provisioner   = (*LeafPEMLoader)(nil)
)
