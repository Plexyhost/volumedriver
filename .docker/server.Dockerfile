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
RUN go build -o /bin/storage-server ./cmd/server

# -------
# Runtime
# -------

FROM alpine:latest

RUN mkdir -p /data
WORKDIR /data
COPY --from=builder /bin/storage-server /bin/storage-server
RUN chmod +x /bin/storage-server

EXPOSE 3000
ENTRYPOINT ["/bin/storage-server"]
