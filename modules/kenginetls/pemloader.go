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

	"github.com/khulnasoft/kengine/v2"
)

func init() {
	kengine.RegisterModule(PEMLoader{})
}

// PEMLoader loads certificates and their associated keys by
// decoding their PEM blocks directly. This has the advantage
// of not needing to store them on disk at all.
type PEMLoader []CertKeyPEMPair

// Provision implements kengine.Provisioner.
func (pl PEMLoader) Provision(ctx kengine.Context) error {
	repl, ok := ctx.Value(kengine.ReplacerCtxKey).(*kengine.Replacer)
	if !ok {
		repl = kengine.NewReplacer()
	}
	for k, pair := range pl {
		for i, tag := range pair.Tags {
			pair.Tags[i] = repl.ReplaceKnown(tag, "")
		}
		pl[k] = CertKeyPEMPair{
			CertificatePEM: repl.ReplaceKnown(pair.CertificatePEM, ""),
			KeyPEM:         repl.ReplaceKnown(pair.KeyPEM, ""),
			Tags:           pair.Tags,
		}
	}
	return nil
}

// KengineModule returns the Kengine module information.
func (PEMLoader) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "tls.certificates.load_pem",
		New: func() kengine.Module { return new(PEMLoader) },
	}
}

// CertKeyPEMPair pairs certificate and key PEM blocks.
type CertKeyPEMPair struct {
	// The certificate (public key) in PEM format.
	CertificatePEM string `json:"certificate"`

	// The private key in PEM format.
	KeyPEM string `json:"key"`

	// Arbitrary values to associate with this certificate.
	// Can be useful when you want to select a particular
	// certificate when there may be multiple valid candidates.
	Tags []string `json:"tags,omitempty"`
}

// LoadCertificates returns the certificates contained in pl.
func (pl PEMLoader) LoadCertificates() ([]Certificate, error) {
	certs := make([]Certificate, 0, len(pl))
	for i, pair := range pl {
		cert, err := tls.X509KeyPair([]byte(pair.CertificatePEM), []byte(pair.KeyPEM))
		if err != nil {
			return nil, fmt.Errorf("PEM pair %d: %v", i, err)
		}
		certs = append(certs, Certificate{
			Certificate: cert,
			Tags:        pair.Tags,
		})
	}
	return certs, nil
}

// Interface guard
var (
	_ CertificateLoader   = (PEMLoader)(nil)
	_ kengine.Provisioner = (PEMLoader)(nil)
)
