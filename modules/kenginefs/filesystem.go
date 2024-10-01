package kenginefs

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"go.uber.org/zap"

	"github.com/khulnasoft/kengine/v2"
	"github.com/khulnasoft/kengine/v2/kengineconfig"
	"github.com/khulnasoft/kengine/v2/kengineconfig/httpkenginefile"
	"github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
)

func init() {
	kengine.RegisterModule(Filesystems{})
	httpkenginefile.RegisterGlobalOption("filesystem", parseFilesystems)
}

type moduleEntry struct {
	Key           string          `json:"name,omitempty"`
	FileSystemRaw json.RawMessage `json:"file_system,omitempty" kengine:"namespace=kengine.fs inline_key=backend"`
	fileSystem    fs.FS
}

// Filesystems loads kengine.fs modules into the global filesystem map
type Filesystems struct {
	Filesystems []*moduleEntry `json:"filesystems"`

	defers []func()
}

func parseFilesystems(d *kenginefile.Dispenser, existingVal any) (any, error) {
	p := &Filesystems{}
	current, ok := existingVal.(*Filesystems)
	if ok {
		p = current
	}
	x := &moduleEntry{}
	err := x.UnmarshalKenginefile(d)
	if err != nil {
		return nil, err
	}
	p.Filesystems = append(p.Filesystems, x)
	return p, nil
}

// KengineModule returns the Kengine module information.
func (Filesystems) KengineModule() kengine.ModuleInfo {
	return kengine.ModuleInfo{
		ID:  "kengine.filesystems",
		New: func() kengine.Module { return new(Filesystems) },
	}
}

func (xs *Filesystems) Start() error { return nil }
func (xs *Filesystems) Stop() error  { return nil }

func (xs *Filesystems) Provision(ctx kengine.Context) error {
	// load the filesystem module
	for _, f := range xs.Filesystems {
		if len(f.FileSystemRaw) > 0 {
			mod, err := ctx.LoadModule(f, "FileSystemRaw")
			if err != nil {
				return fmt.Errorf("loading file system module: %v", err)
			}
			f.fileSystem = mod.(fs.FS)
		}
		// register that module
		ctx.Logger().Debug("registering fs", zap.String("fs", f.Key))
		ctx.Filesystems().Register(f.Key, f.fileSystem)
		// remember to unregister the module when we are done
		xs.defers = append(xs.defers, func() {
			ctx.Logger().Debug("unregistering fs", zap.String("fs", f.Key))
			ctx.Filesystems().Unregister(f.Key)
		})
	}
	return nil
}

func (f *Filesystems) Cleanup() error {
	for _, v := range f.defers {
		v()
	}
	return nil
}

func (f *moduleEntry) UnmarshalKenginefile(d *kenginefile.Dispenser) error {
	for d.Next() {
		// key required for now
		if !d.Args(&f.Key) {
			return d.ArgErr()
		}
		// get the module json
		if !d.NextArg() {
			return d.ArgErr()
		}
		name := d.Val()
		modID := "kengine.fs." + name
		unm, err := kenginefile.UnmarshalModule(d, modID)
		if err != nil {
			return err
		}
		fsys, ok := unm.(fs.FS)
		if !ok {
			return d.Errf("module %s (%T) is not a supported file system implementation (requires fs.FS)", modID, unm)
		}
		f.FileSystemRaw = kengineconfig.JSONModuleObject(fsys, "backend", name, nil)
	}
	return nil
}
