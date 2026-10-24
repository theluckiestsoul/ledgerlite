# Ledgerlite

A small wallet and ledger REST API.

**Tech:** Go 1.22+, standard library only (`net/http` routing, `encoding/json`, `crypto`). No external dependencies.

## Run

```bash
make run      # or: go run ./cmd/ledgerlite   (http://localhost:8080)
make check    # go vet + go test -race
```

Configure API keys with `LEDGERLITE_API_KEYS="key1:user1,key2:user2"` (demo keys are used unless `APP_ENV=production`). `PORT` defaults to 8080.

## Endpoints

All endpoints except `/health` need `Authorization: Bearer <api key>`. Amounts are integers in minor units (`12345` = ₹123.45).

- `GET /health`
- `POST /accounts`, `GET /accounts`, `GET /accounts/{id}`
- `POST /accounts/{id}/deposits`, `POST /accounts/{id}/withdrawals`

Validation errors return every problem in `details.errors`; `details.field` still names the first one.
