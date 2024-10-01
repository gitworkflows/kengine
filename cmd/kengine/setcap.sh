#!/bin/sh

# USAGE:
# 	go run -exec ./setcap.sh main.go <args...>
#
# (Example: `go run -exec ./setcap.sh main.go run --config kengine.json`)
#
# For some reason this does not work on my Arch system, so if you find that's
# the case, you can instead do:
#
# 	go build && ./setcap.sh ./kengine <args...>
#
# but this will leave the ./kengine binary laying around.
#

sudo setcap cap_net_bind_service=+ep "$1"
"$@"
