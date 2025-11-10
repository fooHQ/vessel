#!/usr/bin/env bash

set -euo pipefail

test() {
    WITH_RACE="-race"
    export CGO_ENABLED=1
    go test -tags "dev" $WITH_RACE -timeout 30s ./...
}

build() {
    WITH_LDFLAGS="$(agent_config)"
    if [ "$GOOS" = "windows" ]; then
        WITH_LDFLAGS="$WITH_LDFLAGS -H windowsgui"
    fi
    go build -tags "$TAGS" -o "$OUTPUT" -ldflags "$WITH_LDFLAGS" ./cmd/vessel
}

agent_config() {
   cat <<EOF | tr '\n' ' '
-X main.ID=$ID
-X main.Server=$SERVER
-X main.ServerCertificate=$SERVER_CERTIFICATE
-X main.UserJWT=$USER_JWT
-X main.UserKey=$USER_KEY
-X main.Stream=$STREAM
-X main.Consumer=$CONSUMER
-X main.InboxPrefix=$INBOX_PREFIX
-X main.ObjectStoreName=$OBJECT_STORE_NAME
-X main.ReconnectInterval=$RECONNECT_INTERVAL
-X main.ReconnectJitter=$RECONNECT_JITTER
-X main.AwaitMessagesDuration=$AWAIT_MESSAGES_DURATION
EOF
}

eval $@
