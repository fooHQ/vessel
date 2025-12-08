#!/usr/bin/env bash

set -euo pipefail

test() {
    WITH_RACE="-race"
    CGO_ENABLED=1 go test -tags "dev" $WITH_RACE -timeout 30s ./...
}

build() {
    if [ ! -f "$FJ_SERVER_CERTIFICATE" ]; then
        echo "$0: certificate '$FJ_SERVER_CERTIFICATE' not found"
        exit 1
    fi

    WITH_LDFLAGS="$(cat <<EOF | tr '\n' ' '
-X main.AgentID=$FJ_AGENT_ID
-X main.ServerURL=$FJ_SERVER_URL
-X main.ServerCertificate=$(base64 -w 0 < "$FJ_SERVER_CERTIFICATE")
-X main.UserJWT=$FJ_USER_JWT
-X main.UserKey=$FJ_USER_KEY
-X main.Stream=$FJ_STREAM
-X main.Consumer=$FJ_CONSUMER
-X main.InboxPrefix=$FJ_INBOX_PREFIX
-X main.ObjectStore=$FJ_OBJECT_STORE
-X main.AwaitMessagesDuration=$VESSEL_AWAIT_MESSAGES_DURATION
-X main.IdleDuration=$VESSEL_IDLE_DURATION
-X main.IdleJitter=$VESSEL_IDLE_JITTER
EOF
)"

    if [ "$OS" = "windows" ]; then
        WITH_LDFLAGS="$WITH_LDFLAGS -H windowsgui"
    fi

    GOOS="$OS" GOARCH="$ARCH" go build -tags "$FEATURES" -o "$TARGET" -ldflags "$WITH_LDFLAGS" ./cmd/vessel
}

eval $@
