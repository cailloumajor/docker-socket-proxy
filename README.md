# Docker socket proxy

Exposes a Docker HTTP endpoint that filters and proxies requests to Docker socket.

The intended use of this service is to expose Docker API to other containers without giving them access to the Docker socket. The API is exposed on a listening TCP port.

## Goals

* Run as unprivileged as possible (non-root user);
* Packaged in a Docker image without shell;
* Lightweight, only code for required features.

## Usage

This service is provided as a Docker image, configuration options can be provided by environment variables.

```ShellSession
```
