# Apprize

A drop-in, single-binary reimplementation of the [caronc/apprise-api](https://github.com/caronc/apprise-api)
HTTP API in Go. It speaks the same routes and response shapes as Apprise API
`swagger.yaml` v1.5.0, backed by [unraid/apprise-go](https://github.com/unraid/apprise-go)
for delivery and [modernc.org/sqlite](https://modernc.org/sqlite) for storage —
pure Go, no cgo, no Python runtime.

## Build & run

Requires Go 1.26+.

```sh
go build
./apprize --bind :8000 --db ./apprize.db
```

Ephemeral (no file persistence):

```sh
./apprize --db :memory:
```

### Docker

```sh
docker build -t apprize .
docker run --rm -p 8000:8000 -v "$PWD/data:/data" apprize
```

The image runs `apprize` directly and defaults to:

- `APPRIZE_BIND=:8000`
- `APPRIZE_DB_PATH=/data/apprize.db`

## Configuration

Flags override environment variables. Names use the `APPRIZE_` prefix; the
upstream `HTTP_PORT` is also honoured for the listen port.

| Env                             | Flag                  | Default        | Purpose                                       |
| ------------------------------- | --------------------- | -------------- | --------------------------------------------- |
| `APPRIZE_BIND` (or `HTTP_PORT`) | `--bind`              | `:8000`        | Listen address                                |
| `APPRIZE_DB_PATH`               | `--db`                | `./apprize.db` | SQLite path; `:memory:` for ephemeral         |
| `APPRIZE_API_KEY`               | `--api-key`           | _(none)_       | Enables simple auth only when set             |
| `APPRIZE_STATELESS_URLS`        | —                     | _(none)_       | Default URLs for `POST /notify`               |
| `APPRIZE_CONFIG_LOCK`           | —                     | `no`           | Reject config writes with `403`               |
| `APPRIZE_ADMIN`                 | —                     | `no`           | Allow `GET /cfg` listing                      |
| `APPRIZE_RECURSION_MAX`         | —                     | `1`            | Inbound recursion limit                       |
| `APPRIZE_DENY_SERVICES`         | —                     | _(none)_       | Schemas to reject (comma/space separated)     |
| `APPRIZE_ALLOW_SERVICES`        | —                     | _(none)_       | Allow-list of schemas (exclusive when set)    |
| `APPRIZE_CONFIG_MAX_LENGTH`     | `--config-max-length` | `512` (KB)     | Request body limit                            |
| `APPRIZE_DEFAULT_CONFIG_ID`     | `--default-config-id` | `apprise`      | Default key used by keyless persistent routes |

## API

Routes match the Apprise API contract (`testdata/swagger.yaml`):

| Method & path                                         | Purpose                                                |
| ----------------------------------------------------- | ------------------------------------------------------ |
| `GET /status`                                         | Server status                                          |
| `GET /details`                                        | Version and supported schemas                          |
| `POST /notify`                                        | Stateless notification                                 |
| `POST /add/{key}`                                     | Store a named configuration                            |
| `POST /del/{key}`                                     | Delete a configuration                                 |
| `POST /get/{key}` · `POST /cfg/{key}`                 | Fetch a configuration                                  |
| `POST /add` · `POST /del` · `POST /get` · `POST /cfg` | Same as keyed routes using `APPRIZE_DEFAULT_CONFIG_ID` |
| `GET /cfg`                                            | List configuration keys (requires `APPRIZE_ADMIN`)     |
| `POST /notify/{key}`                                  | Notify using a stored configuration                    |
| `GET /json/urls/{key}`                                | List a configuration's URLs as JSON                    |

## Limitations

apprize intentionally diverges from upstream where apprise-go cannot match it:

- **Attachments are not delivered** — accepted then logged as unsupported;
  `/status` reports `attach_lock=true`.
- **No recursion-header propagation** — inbound `X-Apprise-Recursion-Count` is
  enforced, but it is not injected into outbound requests.
- **`/details` is simplified** — returns supported schemas and version, not
  per-service templates.
- **No web UI** — API only.

## License

[GNU AGPL-3.0](LICENSE)
