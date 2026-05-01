# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Language and Communication

**IMPORTANT**: Always respond in Turkish (Türkçe) for all communication and explanations, as per the Cursor rule in `.cursor/rules/post-code-change-behavior.mdc`. Technical terms, code, and variable names should remain in English.

## Common Development Commands

### Testing
```bash
# Run all integration tests
make integration

# Product-specific tests
make upload                    # Test product upload
make get-single BARCODE=ABC123 # Test single product retrieval
make get-multiple              # Test multiple product retrieval
make delete DELETE=ABC,DEF     # Test product deletion with comma-separated barcodes

# Run specific integration test
go test ./integration -tags=integration -v -count=1 -run TestName

# Environment variables needed (create .env file from env.example)
SELLER_ID=123456
API_KEY=YOUR_API_KEY
API_SECRET=YOUR_API_SECRET
```

### Building
```bash
# Build the library
go build ./...

# Test compilation
go test -c ./...
```

### Dependencies
```bash
# Download dependencies
go mod download

# Tidy dependencies
go mod tidy
```

## High-Level Architecture

### Core Client Structure
The SDK follows a service-oriented architecture where the main `Client` struct (`trendyol.go`) serves as the entry point, providing access to different service modules:

- **Products**: Product CRUD operations and batch management
- **Orders**: Order processing, shipment tracking, and package management
- **PriceInventory**: Price and inventory synchronization
- **Claims**: Return/claim handling
- **Webhooks**: Webhook subscription management
- **Categories/Brands**: Category and brand lookups
- **Finance**: Financial settlements and invoicing
- **Test**: Test environment operations

### API Communication Pattern
1. All API calls go through the central `Do()` method in `Client` which handles:
   - Rate limiting (configurable, default 60 req/min)
   - Automatic retry with exponential backoff
   - Basic authentication using API key/secret
   - Error parsing and standardization

2. Endpoint resolution uses a flexible system (`endpoints.go`):
   - All endpoints are defined as constants with template strings
   - Can be overridden at runtime using `WithEndpointOverrides()`
   - Base URL can be changed dynamically via `SetBaseURL()`

### Key Design Decisions
- **Endpoint Flexibility**: Since Trendyol may change API endpoints, the SDK allows runtime override of any endpoint template without code changes
- **Service Interfaces**: Each service is defined as an interface, making it easy to mock for testing
- **Context Support**: All methods accept context for cancellation and timeout control
- **Batch Operations**: Product operations return batch IDs that must be checked via `GetBatchStatus()`

### Error Handling
The SDK uses a custom `Error` type that captures:
- HTTP status code
- Error message
- Detailed error items with field-level information

### Testing Strategy
Integration tests require real API credentials and are tagged with `integration` build tag. Tests are designed to:
- Be idempotent where possible
- Use test data prefixes (e.g., "TEST-BARCODE-*")
- Support command-line flags for specific test scenarios

## Important Notes

1. **Sandbox Environment**: Requires IP whitelisting by Trendyol. Most examples use production mode (`isSandbox=false`)

2. **Product Updates**: The `Update` method only updates product information, not stock/price. Use `PriceInventory.Update()` for stock/price changes

3. **Batch Processing**: Always check batch status after create/update/delete operations using the returned `batchRequestId`

4. **Required Attributes**: Each category has required attributes that must be included when creating/updating products. Use `Categories.GetCategoryAttributes()` to fetch these

5. **API Limits**: 
   - Max 1000 items per batch request
   - Default rate limit: 60 requests/minute
   - Webhook limit: 15 webhooks per seller

6. **Authentication**: Uses HTTP Basic Auth with API key as username and API secret as password