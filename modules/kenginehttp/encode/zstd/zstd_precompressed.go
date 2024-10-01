package kenginezstd

import (
	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/modules/kenginehttp/encode"
)

func init() {
	kengine.RegisterModule(ZstdPrecompressed{})
}

// ZstdPrecompressed provides the file extension for files precompressed with zstandard encoding.
type ZstdPrecompressed struct {
	Zstd
}

// KengineModule returns the Kengine module information.
func (ZstdPrecompressed) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "http.precompressed.zstd",
		New: func() kengine.Module { return new(ZstdPrecompressed) },
	}
}

// Suffix returns the filename suffix of precompressed files.
func (ZstdPrecompressed) Suffix() string { return ".zst" }

var _ encode.Precompressed = (*ZstdPrecompressed)(nil)
