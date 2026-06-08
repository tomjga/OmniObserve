FROM ubuntu:22.04

# Metadata
LABEL maintainer="tomasjga@proton.me"
LABEL description="GitHub Actions self-hosted runner"

# Environment
ENV RUNNER_VERSION=2.325.0
ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl unzip tar git jq ca-certificates libicu70 libssl3 libunwind8 zlib1g libcurl4 \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user
RUN useradd -m -s /bin/bash runner

WORKDIR /home/runner

# Download and extract GitHub runner
RUN mkdir actions-runner && cd actions-runner && \
    curl -L -o actions-runner-linux-arm64-${RUNNER_VERSION}.tar.gz https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-arm64-${RUNNER_VERSION}.tar.gz && \
    tar xzf actions-runner-linux-arm64-${RUNNER_VERSION}.tar.gz && \
    rm actions-runner-linux-arm64-${RUNNER_VERSION}.tar.gz

# Add entrypoint script
COPY entrypoint.sh /home/runner/entrypoint.sh
RUN chown runner:runner /home/runner/entrypoint.sh && chmod +x /home/runner/entrypoint.sh
USER runner

ENTRYPOINT ["/home/runner/entrypoint.sh"]
