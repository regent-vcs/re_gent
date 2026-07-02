FROM golang:1.23-alpine AS builder
COPY . /src
WORKDIR /src
RUN go build -o /rgt ./cmd/rgt

FROM ubuntu:26.04
RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 bash-completion \
    && rm -rf /var/lib/apt/lists/* \
    && echo 'source /etc/bash_completion' >> /root/.bashrc \
    && echo 'source <(rgt completion bash)' >> /root/.bashrc
COPY --from=builder /rgt /usr/local/bin/
COPY examples/bad-refactor/ /demo/
WORKDIR /workspace
RUN /demo/setup.sh /workspace > /dev/null 2>&1
