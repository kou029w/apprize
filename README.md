# Apprize

Apprize is a Go reimplementation of the [caronc/apprise-api](https://github.com/caronc/apprise-api) HTTP API, shipped as a single binary. It serves the same stateless routes and response shapes as Apprise API `swagger.yaml` v2.0.2 and sends notifications through [unraid/apprise-go](https://github.com/unraid/apprise-go). It is pure Go, with no cgo and no Python runtime.

## Quick start

Start the server:

```sh
docker run --rm -p 8000:8000 fogtype/apprize
```

Send a notification:

```sh
# ntfy, view the message at https://ntfy.sh/apprize-demo
curl localhost:8000/notify \
  -d 'urls=ntfys://apprize-demo' \
  -d body=hello

# email, needs real SMTP credentials
curl localhost:8000/notify \
  -d 'urls=mailtos://user:pass@gmail.com' \
  -d body=hello
```

URL formats for other services are listed at [appriseit.com/services](https://appriseit.com/services/).

## Install

```sh
# Docker
docker run --rm -p 8000:8000 fogtype/apprize

# Podman
podman run --rm -p 8000:8000 fogtype/apprize

# Go
go install git.fogtype.com/nebel/apprize@latest
apprize --bind :8000

# Build from source
git clone https://git.fogtype.com/nebel/apprize
cd apprize
go build
./apprize --bind :8000
```

### Docker Compose

```yaml
# compose.yml
services:
  apprize:
    image: fogtype/apprize
    restart: unless-stopped
    ports:
      - "8000:8000"
```

```sh
docker compose up -d
```

## Configuration

Flags override environment variables. Names use the `APPRIZE_` prefix; the
upstream `HTTP_PORT` is also honoured for the listen port.

| Env                             | Flag        | Default | Purpose                                   |
| ------------------------------- | ----------- | ------- | ----------------------------------------- |
| `APPRIZE_BIND` (or `HTTP_PORT`) | `--bind`    | `:8000` | Listen address                            |
| `APPRIZE_API_KEY`               | `--api-key` | -       | Enables simple auth when set              |
| `APPRIZE_STATELESS_URLS`        | -           | -       | Default URLs for `POST /notify`           |
| `APPRIZE_RECURSION_MAX`         | -           | `1`     | Inbound recursion limit                   |
| `APPRIZE_DENY_SERVICES`         | -           | -       | Schemas to reject (comma/space separated) |
| `APPRIZE_ALLOW_SERVICES`        | -           | -       | Accept only these schemas when set        |
| `APPRIZE_DEFAULT_FORMAT`        | -           | -       | `format` used when a request omits it     |

## API

| Method & path  | Purpose                       |
| -------------- | ----------------------------- |
| `GET /status`  | Server status                 |
| `GET /details` | Version and supported schemas |
| `POST /notify` | Stateless notification        |

## Limitations

apprize differs from upstream where apprise-go can't match it, and it leaves
out the persistent-configuration, account, and metrics endpoints.

- `/add`, `/del`, `/get`, `/cfg`, `/move`, `/qr`, `POST /notify/{key}`,
  `GET /status/{key}`, and `GET /json/urls` are not implemented. The server
  stores nothing.
- `/auth`, `/login`, `/logout`, and `/metrics` are not implemented. Upstream
  Basic Auth (`APPRISE_AUTH_REQUIRED`) is not supported; use `APPRIZE_API_KEY`
  instead. `X-Apprise-Config-ID` is ignored.
- `POST /notify` does not stream progress: `?stream=` and
  `Accept: text/event-stream` are ignored, because apprise-go reports results
  only after every target has finished.
- `tag` and `template` values are accepted and validated but have no effect,
  since explicit URLs carry no tags or `${NAME}` markers.
- The server accepts attachments but does not deliver them. It logs them as
  unsupported, and `/status` reports `attach_lock=true`.
- The server enforces inbound `X-Apprise-Recursion-Count` but does not add the
  header to outbound requests.
- `/details` returns the version and supported schemas, without per-service
  templates.
- There is no web UI.

## License

[GNU AGPL-3.0](LICENSE)

## Acknowledgements

- [caronc/apprise-api](https://github.com/caronc/apprise-api) for the original API design and specification.
- [unraid/apprise-go](https://github.com/unraid/apprise-go) for the Go notification library.
