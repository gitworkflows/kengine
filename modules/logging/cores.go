package logging

import (
	"go.uber.org/zap/zapcore"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
)

func init() {
	kengine.RegisterModule(MockCore{})
}

// MockCore is a no-op module, purely for testing
type MockCore struct {
	zapcore.Core `json:"-"`
}

// KengineModule returns the Kengine module information.
func (MockCore) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "kengine.logging.cores.mock",
		New: func() kengine.Module { return new(MockCore) },
	}
}

func (lec *MockCore) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	return nil
}

// Interface guards
var (
	_ zapcore.Core            = (*MockCore)(nil)
	_ kengine.Module          = (*MockCore)(nil)
	_ kenginefile.Unmarshaler = (*MockCore)(nil)
)
