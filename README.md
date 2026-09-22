# MaaS Model Catalog

Small JSON web service for Huawei Cloud MaaS model pricing, limits, and API URLs.

## Endpoints

- `GET /models` returns the model catalog.
- `GET /health` returns `{ "status": "ok" }`.

## Development

```sh
go test ./...
go run .
```

The server listens on `PORT`, defaulting to `3000`.

## Docker

```sh
docker compose up -d --build
```

## Catalog data

The catalog contains only DeepSeek V4.1 Flash, V4 Flash, V4 Pro, and GLM 5.1, 5.2, and 5.3.
Model IDs and limits follow the [Huawei MaaS model list](https://support.huaweicloud.com/intl/en-us/model-list-maas/model_list_0001.html).
Prices follow [Huawei MaaS billing](https://support.huaweicloud.com/intl/en-us/price-maas/price-maas-0002.html), checked September 23, 2026.

The existing input/output pricing fields represent standard, non-cached peak rates in USD per million tokens. They do not encode off-peak discounts or cache-hit pricing.
