package testmocks

import (
	"crypto/x509"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	"github.com/khulnasoft/kengine/v2/modules/kenginetls"
)

func init() {
	kengine.RegisterModule(new(dummyVerifier))
}

type dummyVerifier struct{}

// UnmarshalKenginefile implements kenginefile.Unmarshaler.
func (dummyVerifier) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	return nil
}

// KengineModule implements kengine.Module.
func (dummyVerifier) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID: "tls.client_auth.verifier.dummy",
		New: func() kengine.Module {
			return new(dummyVerifier)
		},
	}
}

// VerifyClientCertificate implements ClientCertificateVerifier.
func (dummyVerifier) VerifyClientCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return nil
}

var (
	_ kengine.Module                       = dummyVerifier{}
	_ kenginetls.ClientCertificateVerifier = dummyVerifier{}
	_ kenginefile.Unmarshaler              = dummyVerifier{}
)
