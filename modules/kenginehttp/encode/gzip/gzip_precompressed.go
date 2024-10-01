package kenginegzip

import (
	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp/encode"
)

func init() {
	kengine.RegisterModule(GzipPrecompressed{})
}

// GzipPrecompressed provides the file extension for files precompressed with gzip encoding.
type GzipPrecompressed struct {
	Gzip
}

// KengineModule returns the Kengine module information.
func (GzipPrecompressed) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.precompressed.gzip",
		New: func() kengine.Module { return new(GzipPrecompressed) },
	}
}

// Suffix returns the filename suffix of precompressed files.
func (GzipPrecompressed) Suffix() string { return ".gz" }

var _ encode.Precompressed = (*GzipPrecompressed)(nil)
