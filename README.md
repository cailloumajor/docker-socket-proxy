# Docker socket proxy

Exposes a Docker HTTP endpoint that filters and proxies requests to Docker socket.

The intended use of this service is to expose Docker API to other containers without giving them access to the Docker socket. The API is exposed on a listening TCP port.

## Goals

* Run as unprivileged as possible (non-root user);
* Packaged in a Docker image without shell;
* Lightweight, only code for required features.

## Configuration file

The configuration is provided as a [TOML](https://toml.io) file, whose path must be given to the according program argument (see below). The file contains following elements.

### Allowing filters (`allow_filters` key)

An array of objects describing the filters that allow requests to pass through. The first matching filter will allow the request, if no filter matches, the request is blocked (returns a HTTP 403 code to the client).

Each filter object is composed as follows, the request must match all of the filter components to be allowed.

| Key      | Value type | Description                           |
| -------- | ---------- | ------------------------------------- |
| `method` | string     | The method to match (case sensitive). |
| `path`   | string     | A path pattern (see below).           |

[Path patterns][patterns] are matched using [`doublestar.Match`][doublestar-match].

#### Notes

* Requests with `HEAD` method are always allowed;
* API versioning prefix will be removed from the request path before matching, so path patterns must omit it.

[doublestar-match]: https://pkg.go.dev/github.com/bmatcuk/doublestar/v4#Match
[patterns]: https://github.com/bmatcuk/doublestar#patterns

## Usage

This service is provided as a Docker image, arguments can be provided by environment variables.

```ShellSession
$ docker-socket-proxy -help
USAGE
  docker-socket-proxy [options]

OPTIONS
  Flag          Env Var      Description
  -api-listen   API_LISTEN   Listen address (default: 127.0.0.1:2375)
  -config-file  CONFIG_FILE  Path to the TOML configuration file
  -socket-file  SOCKET_FILE  Path to the Docker socket file (default: /var/run/docker.sock)
  -verbose                   Be more verbose (default: false)
  -version                   Print version information and exit (default: false)
```
