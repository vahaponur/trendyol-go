# Changelog

## v0.3.0 — 2026-09-15

### Fixed

- Decode string, number, empty and null order-address phones without changing the public `int64` field. This fixes `cannot unmarshal string into Go struct field OrderAddress...phone of type int64` when listing orders.
- Accept numeric ETGB timestamps and numeric-string cargo tracking numbers.
- Read current order-line identifiers, stock codes, prices, VAT and seller discounts into the existing Go fields. Prefer current values, including zero, when responses include both old and new names.
- Preserve older order/webhook payload support and adapt `ListLegacy` to current shipment packages, including addresses and line items.

### Changed

- `Orders.List` and `Orders.ListLegacy` use `/integration/order/sellers/{sellerId}/v2/orders` in production and stage. Endpoint overrides remain supported.

### Added

- `Orders.ListStream`, `ListOrdersStreamOptions`, `StreamResponse` and `EndpointGetOrdersStreamKey` for `/integration/order/sellers/{sellerId}/orders/stream`.
- Order payment method, country, seller discount, invoice metadata, address tax fields, line content ID and total discount.
- Regression tests for current and legacy JSON, pagination, cursor continuation, endpoint overrides, authentication, API errors and the existing cargo-provider endpoint.

`OrderService` now includes `ListStream`; custom implementations and mocks of that interface must implement the new method. Existing client calls and public field types remain supported. Product V2 services use different request/response contracts and are outside this order API release.

Sources: [Order V2](https://developers.trendyol.com/docs/sipariş-paketlerini-çekme-getshipmentpackages), [order stream](https://developers.trendyol.com/docs/sipariş-paketlerini-akış-ile-çekme).
