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
-X main.UserJWT=$USER_JWT
-X main.UserKey=$USER_KEY
-X main.CACertificate=$CA_CERTIFICATE
-X main.Stream=$STREAM
-X main.Consumer=$CONSUMER
-X main.InboxPrefix=$INBOX_PREFIX
-X main.ObjectStoreName=$OBJECT_STORE_NAME
-X main.SubjectApiWorkerStartT=$SUBJECT_API_WORKER_START_T
-X main.SubjectApiWorkerStopT=$SUBJECT_API_WORKER_STOP_T
-X main.SubjectApiWorkerWriteStdinT=$SUBJECT_API_WORKER_WRITE_STDIN_T
-X main.SubjectApiWorkerWriteStdoutT=$SUBJECT_API_WORKER_WRITE_STDOUT_T
-X main.SubjectApiWorkerStatusT=$SUBJECT_API_WORKER_STATUS_T
-X main.SubjectApiConnInfoT=$SUBJECT_API_CONN_INFO_T
-X main.SubjectApiReplyT=$SUBJECT_API_REPLY_T
-X main.ReconnectInterval=$RECONNECT_INTERVAL
-X main.ReconnectJitter=$RECONNECT_JITTER
-X main.AwaitMessagesDuration=$AWAIT_MESSAGES_DURATION
EOF
}

eval $@
