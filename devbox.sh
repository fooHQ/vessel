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
-X main.AgentID=$FJ_AGENT_ID
-X main.ServerURL=$FJ_SERVER_URL
-X main.ServerCertificate=$FJ_SERVER_CERTIFICATE
-X main.UserJWT=$FJ_USER_JWT
-X main.UserKey=$FJ_USER_KEY
-X main.Stream=$FJ_STREAM
-X main.Consumer=$FJ_CONSUMER
-X main.InboxPrefix=$FJ_INBOX_PREFIX
-X main.ObjectStore=$FJ_OBJECT_STORE
-X main.AwaitMessagesDuration=$FJ_AWAIT_MESSAGES_DURATION
-X main.IdleDuration=$FJ_IDLE_DURATION
-X main.IdleJitter=$FJ_IDLE_JITTER
EOF
}

eval $@
