package standard

import (
	// standard Kengine HTTP app modules
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/encode"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/encode/brotli"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/encode/gzip"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/encode/zstd"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/fileserver"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/headers"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/intercept"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/kengineauth"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/logging"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/map"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/proxyprotocol"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/push"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/requestbody"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/reverseproxy"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/reverseproxy/fastcgi"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/reverseproxy/forwardauth"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/rewrite"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/templates"
	_ "github.com/khulnasoft/kengine/v2/modules/kenginehttp/tracing"
)
