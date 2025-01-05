# -------
# Builder
# -------

FROM golang:1.23-alpine AS builder
WORKDIR /go/src/github.com/plexyhost/volume-driver

# Deps
COPY go.* .
RUN go mod download -x

# Binary
COPY . .
RUN go build -o /usr/bin/plexhost-volume-plugin ./cmd/driver

# -------
# Runtime
# -------

FROM alpine:latest
RUN mkdir -p /live
COPY --from=builder /usr/bin/plexhost-volume-plugin /usr/bin/plexhost-volume-plugin
RUN chmod +x /usr/bin/plexhost-volume-plugin
ENTRYPOINT ["/usr/bin/plexhost-volume-plugin"]
