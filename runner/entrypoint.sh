#!/bin/bash
set -e

cd /home/runner/actions-runner

# Config with env vars
./config.sh --url "$REPO_URL" --token "$RUNNER_TOKEN" --unattended --replace

# Run the runner (never exits unless stopped)
exec ./run.sh
