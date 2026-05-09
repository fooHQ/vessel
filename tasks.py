#!/usr/bin/env python3

import argparse
import base64
import os
import subprocess
import sys

def build():
    cert = os.environ["FJ_SERVER_CERTIFICATE"]

    if not cert:
        cert = "/dev/null"

    if os.path.isdir(cert):
        print(f"{sys.argv[0]}: certificate file error: is a directory", file=sys.stderr)
        sys.exit(1)
    elif not os.path.exists(cert):
        print(f"{sys.argv[0]}: certificate '{cert}' not found", file=sys.stderr)
        sys.exit(1)

    with open(cert, "rb") as f:
        cert_b64 = base64.b64encode(f.read()).decode("ascii")

    target = os.environ["TARGET"]
    go_os = os.environ["OS"]
    arch = os.environ["ARCH"]
    features = os.environ["FEATURES"]

    ldflags_parts = [
        f"-s -w",
        f"-X main.AgentID={os.environ['FJ_AGENT_ID']}",
        f"-X main.ServerURL={os.environ['FJ_SERVER_URL']}",
        f"-X main.ServerCertificate={cert_b64}",
        f"-X main.UserJWT={os.environ['FJ_USER_JWT']}",
        f"-X main.UserKey={os.environ['FJ_USER_KEY']}",
        f"-X main.Stream={os.environ['FJ_STREAM']}",
        f"-X main.Consumer={os.environ['FJ_CONSUMER']}",
        f"-X main.InboxPrefix={os.environ['FJ_INBOX_PREFIX']}",
        f"-X main.ObjectStore={os.environ['FJ_OBJECT_STORE']}",
        f"-X main.AwaitMessagesDuration={os.environ['VESSEL_AWAIT_MESSAGES_DURATION']}",
        f"-X main.IdleDuration={os.environ['VESSEL_IDLE_DURATION']}",
        f"-X main.IdleJitter={os.environ['VESSEL_IDLE_JITTER']}",
    ]

    if go_os == "windows":
        ldflags_parts.append("-H windowsgui")

    ldflags = " ".join(ldflags_parts)

    cmd = [
        "go", "build",
        "-tags", features,
        "-o", target,
        "-ldflags", ldflags,
        "./cmd/vessel",
    ]

    env = os.environ.copy()
    env["GOOS"] = go_os
    env["GOARCH"] = arch

    result = subprocess.run(cmd, env=env)
    sys.exit(result.returncode)

def test():
    env = os.environ.copy()
    env["CGO_ENABLED"] = "1"

    cmd = [
        "go", "test",
        "-tags", "dev",
        "-race",
        "-timeout", "30s",
        "./...",
    ]

    result = subprocess.run(cmd, env=env)
    sys.exit(result.returncode)

def lint():
    result = subprocess.run(["golangci-lint", "run", "--timeout", "10m"])
    sys.exit(result.returncode)

if __name__ == "__main__":
    choices = [
        "build",
        "test",
        "lint",
    ]
    parser = argparse.ArgumentParser(description="Run project tasks.")
    parser.add_argument("function", choices=choices, help="The function to run.")
    args = parser.parse_args()

    globals()[args.function]()
