# PlexHost Volume Driver

A Docker volume plugin that enables persistent storage synchronization for PlexHost Server containers.

## Features

- Automatic synchronization of PlexHost server data
- Periodic background saves (every 4 minutes)
- HTTP-based storage backend
- Docker plugin interface

## Installation

Install the plugin from GitHub Container Registry:

```bash
docker plugin install ghcr.io/plexyhost/plexhost-driver:latest \
  --grant-all-permissions \
  ENDPOINT=http://example-storage-server
```

## Usage

Servers orchestrated by our backend will look for the plugin. It will automatically be used when deploying.

## Configuration

The plugin accepts the following environment variables:

- `ENDPOINT`: URL of your storage server (required)

## Architecture

The system consists of two main components:

1. **Volume Driver**: Implements Docker's volume plugin interface
2. **Storage Server**: HTTP server that handles data persistence

Data is automatically compressed via the z-standard algorithm before being sent to storage, and decompressed when retrieved.

## Building from Source

1. Clone the repository
2. Build the plugin:

```bash
make clean rootfs create
```

3. Enable the plugin:

```bash
make enable
```
