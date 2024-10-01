package kenginebrotli

import (
	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp/encode"
)

func init() {
	kengine.RegisterModule(BrotliPrecompressed{})
}

// BrotliPrecompressed provides the file extension for files precompressed with brotli encoding.
type BrotliPrecompressed struct{}

// KengineModule returns the Kengine module information.
func (BrotliPrecompressed) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.precompressed.br",
		New: func() kengine.Module { return new(BrotliPrecompressed) },
	}
}

// AcceptEncoding returns the name of the encoding as
// used in the Accept-Encoding request headers.
func (BrotliPrecompressed) AcceptEncoding() string { return "br" }

// Suffix returns the filename suffix of precompressed files.
func (BrotliPrecompressed) Suffix() string { return ".br" }

// Interface guards
var _ encode.Precompressed = (*BrotliPrecompressed)(nil)
