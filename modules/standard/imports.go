package standard

import (
	// standard Kengine modules
	_ "github.com/khulnasoft/kengine/v2/kengineconfig/kenginefile"
	_ "github.com/khulnasoft/kengine/v2/modules/filestorage"
	_ "github.com/khulnasoft/kengine/v2/modules/kengineevents"
	_ "github.com/khulnasoft/kengine/v2/modules/kengineevents/eventsconfig"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginefs"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/standard"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginepki"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginepki/acmeserver"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginetls"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginetls/distributedstek"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginetls/standardstek"
	_ "github.com/khulnasoft/kengine/v2/modules/logging"
	_ "github.com/khulnasoft/kengine/v2/modules/metrics"
)
