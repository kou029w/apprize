# Apprize

A drop-in, single-binary reimplementation of the [caronc/apprise-api](https://github.com/caronc/apprise-api) HTTP API in Go. Speaks the same stateless routes and response shapes as Apprise API `swagger.yaml` v1.5.0, backed by [unraid/apprise-go](https://github.com/unraid/apprise-go) for delivery — pure Go, no cgo, no Python runtime.

## Quick start

### Docker Hub

```sh
docker run --rm -p 8000:8000 fogtype/apprize:latest
```

### Go install

```sh
go install git.fogtype.com/nebel/apprize@latest
apprize --bind :8000
```

### Build from source

```sh
go build
./apprize --bind :8000
```

## Configuration

Flags override environment variables. Names use the `APPRIZE_` prefix; the
upstream `HTTP_PORT` is also honoured for the listen port.

| Env                             | Flag        | Default  | Purpose                                    |
| ------------------------------- | ----------- | -------- | ------------------------------------------ |
| `APPRIZE_BIND` (or `HTTP_PORT`) | `--bind`    | `:8000`  | Listen address                             |
| `APPRIZE_API_KEY`               | `--api-key` | _(none)_ | Enables simple auth only when set          |
| `APPRIZE_STATELESS_URLS`        | —           | _(none)_ | Default URLs for `POST /notify`            |
| `APPRIZE_RECURSION_MAX`         | —           | `1`      | Inbound recursion limit                    |
| `APPRIZE_DENY_SERVICES`         | —           | _(none)_ | Schemas to reject (comma/space separated)  |
| `APPRIZE_ALLOW_SERVICES`        | —           | _(none)_ | Allow-list of schemas (exclusive when set) |

## API

| Method & path  | Purpose                       |
| -------------- | ----------------------------- |
| `GET /status`  | Server status                 |
| `GET /details` | Version and supported schemas |
| `POST /notify` | Stateless notification        |

## Limitations

apprize intentionally diverges from upstream where apprise-go cannot match it,
and omits the persistent-configuration half of the API entirely:

- **No persistent config endpoints** — `/add`, `/del`, `/get`, `/cfg`,
  `POST /notify/{key}`, and `GET /json/urls/{key}` are not implemented.
  There is no storage layer; the server is fully stateless.
- **Attachments are not delivered** — accepted then logged as unsupported;
  `/status` reports `attach_lock=true`.
- **No recursion-header propagation** — inbound `X-Apprise-Recursion-Count` is
  enforced, but it is not injected into outbound requests.
- **`/details` is simplified** — returns supported schemas and version, not
  per-service templates.
- **No web UI** — API only.

## License

[GNU AGPL-3.0](LICENSE)

## Acknowledgements

- [caronc/apprise-api](https://github.com/caronc/apprise-api) for the original API design and specification.
- [unraid/apprise-go](https://github.com/unraid/apprise-go) for the Go notification library.
