# zot-docker-proxy

[![Release](https://github.com/USA-RedDragon/zot-docker-proxy/actions/workflows/release.yaml/badge.svg)](https://github.com/USA-RedDragon/zot-docker-proxy/actions/workflows/release.yaml) [![go.mod version](https://img.shields.io/github/go-mod/go-version/USA-RedDragon/zot-docker-proxy.svg)](https://github.com/USA-RedDragon/zot-docker-proxy) [![GoReportCard](https://goreportcard.com/badge/github.com/USA-RedDragon/zot-docker-proxy)](https://goreportcard.com/report/github.com/USA-RedDragon/zot-docker-proxy) [![License](https://badgen.net/github/license/USA-RedDragon/zot-docker-proxy)](https://github.com/USA-RedDragon/zot-docker-proxy/blob/main/LICENSE) [![Release](https://img.shields.io/github/release/USA-RedDragon/zot-docker-proxy.svg)](https://github.com/USA-RedDragon/zot-docker-proxy/releases/) [![codecov](https://codecov.io/gh/USA-RedDragon/zot-docker-proxy/graph/badge.svg?token=J73cSjZcIG)](https://codecov.io/gh/USA-RedDragon/zot-docker-proxy)

A simple proxy server for [Zot](https://zotregistry.dev) to enable use of the Docker CLI. This is to work around [Zot issue 2928](https://github.com/project-zot/zot/issues/2928#issuecomment-2641225960), as Zot will not add support for the Docker CLI.

Huge shoutout to [@gabe565](https://github.com/gabe565) for the original implementation of this proxy server and discovering the changes needed to support the Docker CLI.

## How it works

If the user agent does not begin with `docker/`, requests will be forwarded to Zot unmodified.

The Docker CLI relies on the registry to redirect it to a token service in the case that it sends a request to `/v2` without authentication. This project, by way of reverse proxying, provides a `/docker-token` endpoint which provides the anonymous token that the Docker CLI requests. When future requests to the API come in from the Docker CLI, this proxy will validate the token, then if valid, forward it as an anonymous API call to Zot.

This satisfies the authentication requirements for the Docker CLI to work with the Zot registry when anonymous access is allowed.

## Usage

The proxy server is configured either by command line flags, a configuration file, or environment variables. All three methods can be used together, with command line flags taking precedence over environment variables, which take precedence over the configuration file.

### Configuration Options

|           Flag           |      Env Variable      |   Config File Option   |                                                Description                                                |          Default           |
| ------------------------ | ---------------------- | ---------------------- | --------------------------------------------------------------------------------------------------------- | -------------------------- |
| `--log-level`            | `LOG_LEVEL`            | `log-level`            | The log level to use. Options are `debug`, `info`, `warn`, `error`.                                       | `info`                     |
| `--port`                 | `PORT`                 | `port`                 | The port to listen on for incoming connections.                                                           | `8080`                     |
| `--secret`               | `SECRET`               | `secret`               | Secret used to sign tokens, required.                                                                     | None (must specify)        |
| `--zot-url`              | `ZOT_URL`              | `zot-url`              | The URL of the Zot registry to proxy requests to. Must be specified.                                      | None (must specify)        |
| `--my-url`               | `MY_URL`               | `my-url`               | The URL of this zot-docker-proxy instance. Used in the token service to generate URLs. Must be specified. | None (must specify)        |
| `--cors-allowed-origins` | `CORS_ALLOWED_ORIGINS` | `cors-allowed-origins` | A list of allowed origins for CORS. If not specified, all origins are allowed.                            | `["https://*","http://*"]` |
| `--config`               | `CONFIG`               | N/A                    | The path to the configuration file.                                                                       | `config.yaml`              |

### Minimal Example Configuration File

```yaml
zot-url: https://zot.example.com
my-url: http://localhost:8080
secret: change-me
```

### Running with Docker

```bash
docker run -d \
    -p 8080:8080 \
    -e ZOT_URL=https://zot.example.com \
    -e MY_URL=http://localhost:8080 \
    -e SECRET=change-me \
    --name zot-docker-proxy \
    ghcr.io/usa-reddragon/zot-docker-proxy:latest
```

### Building from Source

```bash
git clone https://github.com/USA-RedDragon/zot-docker-proxy.git
cd zot-docker-proxy
go build .
./zot-docker-proxy --zot-url https://zot.example.com --my-url http://localhost:8080 --secret change-me
```

### Kubernetes

I do not provide a Helm chart, however you can observe my Zot manifests in my personal Flux repo as an example: <https://github.com/USA-RedDragon/home-cluster-flux/blob/main/services/zot/app/values.yaml>. This deployment uses the [bjw-s-labs app-template Helm chart](https://bjw-s-labs.github.io/helm-charts/docs/app-template/).
