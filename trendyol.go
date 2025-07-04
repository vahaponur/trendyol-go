// Package trendyol provides a Go client library for the Trendyol Marketplace API.
//
// The client supports all major Trendyol API operations including:
//   - Product management (create, update, list)
//   - Order processing and shipment tracking
//   - Price and inventory synchronization
//   - Returns and claims handling
//   - Category and brand lookups
//
// Example usage:
//
//	client := trendyol.NewClient("123456", "api-key", "api-secret", false)
//
//	// Test authentication
//	err := client.TestAuthentication(context.Background())
//	if err != nil {
//	    log.Fatal("Authentication failed:", err)
//	}
//
//	// Create a product
//	product := trendyol.Product{
//	    Barcode:       "ABC-001",
//	    Title:         "Cotton Hoodie",
//	    ProductMainID: "HOOD-001",
//	    BrandID:       1791,
//	    CategoryID:    411,
//	    Quantity:      100,
//	    StockCode:     "STK-001",
//	    ListPrice:     250.99,
//	    SalePrice:     120.99,
//	    CurrencyType:  "TRY",
//	    VATRate:       18,
//	    Images: []trendyol.ProductImage{
//	        {URL: "https://example.com/image.jpg"},
//	    },
//	}
//
//	batch, err := client.Products.Create(context.Background(), []trendyol.Product{product})
//	if err != nil {
//	    log.Fatal("Failed to create product:", err)
//	}
package trendyol

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Environment constants
const (
	ProdBaseURL    = "https://apigw.trendyol.com"
	SandboxBaseURL = "https://stageapigw.trendyol.com"
)

// Package status constants
const (
	StatusAwaiting  = "Awaiting"
	StatusCreated   = "Created"
	StatusPicking   = "Picking"
	StatusInvoiced  = "Invoiced"
	StatusShipped   = "Shipped"
	StatusDelivered = "Delivered"
	StatusCancelled = "Cancelled"
	StatusUnpacked  = "Unpacked"
	StatusReturned  = "Returned"
)

// Error codes
const (
	ErrCodeValidation     = "VALIDATION_ERROR"
	ErrCodeAuthentication = "AUTHENTICATION_ERROR"
	ErrCodeRateLimit      = "RATE_LIMIT_EXCEEDED"
	ErrCodeNotFound       = "NOT_FOUND"
	ErrCodeInternal       = "INTERNAL_ERROR"
)

// ClientOption is a functional option for configuring the client
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithRetryConfig sets retry configuration
func WithRetryConfig(maxRetries int, retryDelay time.Duration) ClientOption {
	return func(c *Client) {
		c.maxRetries = maxRetries
		c.retryDelay = retryDelay
	}
}

// WithRateLimit sets rate limiting configuration
func WithRateLimit(requestsPerMinute int) ClientOption {
	return func(c *Client) {
		c.rateLimiter = newRateLimiter(requestsPerMinute)
	}
}

// WithUserAgent sets a custom user agent
func WithUserAgent(userAgent string) ClientOption {
	return func(c *Client) {
		c.userAgent = userAgent
	}
}

// Client represents the Trendyol API client
type Client struct {
	baseURL     string
	sellerID    string
	apiKey      string
	apiSecret   string
	userAgent   string
	httpClient  *http.Client
	maxRetries  int
	retryDelay  time.Duration
	rateLimiter *rateLimiter

	endpoints map[string]string // endpoint overrides

	// Service interfaces
	Products          ProductService
	Orders            OrderService
	PriceInventory    PriceInventoryService
	Claims            ClaimService
	Addresses         AddressService
	Categories        CategoryService
	Finance           FinanceService
	CommonLabel       CommonLabelService
	Member            MemberService
	Test              TestService
	ShipmentProviders ShipmentProviderService
}

// NewClient creates a new Trendyol API client with the provided credentials
func NewClient(sellerID, apiKey, apiSecret string, isSandbox bool, opts ...ClientOption) *Client {
	baseURL := ProdBaseURL
	if isSandbox {
		baseURL = SandboxBaseURL
	}

	c := &Client{
		baseURL:   baseURL,
		sellerID:  sellerID,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		userAgent: fmt.Sprintf("%s - SelfIntegration", sellerID),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxRetries:  3,
		retryDelay:  time.Second,
		rateLimiter: newRateLimiter(60), // Default 60 requests per minute
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	// Initialize services
	c.Products = &productService{client: c}
	c.Orders = &orderService{client: c}
	c.PriceInventory = &priceInventoryService{client: c}
	c.Claims = &claimService{client: c}
	c.Addresses = &addressService{client: c}
	c.Categories = &categoryService{client: c}
	c.Finance = &financeService{client: c}
	c.CommonLabel = &commonLabelService{client: c}
	c.Member = &memberService{client: c}
	c.Test = &testService{client: c}
	c.ShipmentProviders = &shipmentProviderService{client: c}

	return c
}

// rateLimiter implements a simple token bucket rate limiter
type rateLimiter struct {
	tokens    int
	maxTokens int
	mu        sync.Mutex
	ticker    *time.Ticker
}

func newRateLimiter(requestsPerMinute int) *rateLimiter {
	rl := &rateLimiter{
		tokens:    requestsPerMinute,
		maxTokens: requestsPerMinute,
		ticker:    time.NewTicker(time.Minute / time.Duration(requestsPerMinute)),
	}

	go func() {
		for range rl.ticker.C {
			rl.mu.Lock()
			if rl.tokens < rl.maxTokens {
				rl.tokens++
			}
			rl.mu.Unlock()
		}
	}()

	return rl
}

func (rl *rateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mu.Lock()
		if rl.tokens > 0 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}
		rl.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Check again
		}
	}
}

// Request represents an API request configuration
type Request struct {
	Method      string
	Path        string
	Query       url.Values
	Body        interface{}
	Result      interface{}
	RawResponse bool
}

// Error represents a Trendyol API error
type Error struct {
	StatusCode int         `json:"statusCode,omitempty"`
	Status     string      `json:"status,omitempty"`
	Message    string      `json:"message,omitempty"`
	Errors     []ErrorItem `json:"errors,omitempty"`
}

// ErrorItem represents a single error in the errors array
type ErrorItem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func (e *Error) Error() string {
	if len(e.Errors) > 0 {
		var msgs []string
		for _, err := range e.Errors {
			if err.Field != "" {
				msgs = append(msgs, fmt.Sprintf("%s: %s (field: %s)", err.Code, err.Message, err.Field))
			} else {
				msgs = append(msgs, fmt.Sprintf("%s: %s", err.Code, err.Message))
			}
		}
		return fmt.Sprintf("Trendyol API Error (%d): %s", e.StatusCode, strings.Join(msgs, "; "))
	}
	return fmt.Sprintf("Trendyol API Error (%d): %s", e.StatusCode, e.Message)
}

// Do executes an API request with automatic retry and rate limiting
func (c *Client) Do(ctx context.Context, req *Request) error {
	// Rate limiting
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit wait failed: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := c.retryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := c.doRequest(ctx, req)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if apiErr, ok := err.(*Error); ok {
			// Don't retry client errors (4xx) except rate limit
			if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 && apiErr.StatusCode != 429 {
				return err
			}
		}
	}

	return fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

func (c *Client) doRequest(ctx context.Context, req *Request) error {
	// Build URL
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	// Use path as-is from resolve() - no hardcoded logic
	path := req.Path
	u.Path = strings.TrimSuffix(u.Path, "/") + path

	// Add query parameters
	if req.Query != nil {
		u.RawQuery = req.Query.Encode()
	}

	// Prepare body
	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, u.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	auth := base64.StdEncoding.EncodeToString([]byte(c.apiKey + ":" + c.apiSecret))
	httpReq.Header.Set("Authorization", "Basic "+auth)
	httpReq.Header.Set("User-Agent", c.userAgent)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle errors
	if resp.StatusCode >= 400 {
		var apiErr Error
		apiErr.StatusCode = resp.StatusCode

		// Try to parse error response
		if err := json.Unmarshal(body, &apiErr); err != nil {
			// Fallback for non-standard error responses
			apiErr.Message = string(body)
		}

		return &apiErr
	}

	// Parse successful response
	if req.Result != nil && !req.RawResponse {
		if err := json.Unmarshal(body, req.Result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	} else if req.RawResponse && req.Result != nil {
		// For raw response, store the body as []byte
		if bytesPtr, ok := req.Result.(*[]byte); ok {
			*bytesPtr = body
		}
	}

	return nil
}

// Pagination represents pagination parameters
type Pagination struct {
	Page int `json:"page"`
	Size int `json:"size"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Page         int `json:"page"`
	Size         int `json:"size"`
	TotalPages   int `json:"totalPages"`
	TotalElement int `json:"totalElements"`
}

// Product represents a product
type Product struct {
	Barcode       string `json:"barcode"`
	Title         string `json:"title"`
	ProductMainID string `json:"productMainId"`
	// New fields from listing API
	Approved            bool               `json:"approved,omitempty"`
	Archived            bool               `json:"archived,omitempty"`
	Brand               string             `json:"brand,omitempty"`
	BrandID             int                `json:"brandId"`
	CategoryID          int                `json:"categoryId"`
	CategoryName        string             `json:"categoryName,omitempty"`
	PimCategoryID       int                `json:"pimCategoryId,omitempty"`
	CreateDateTime      int64              `json:"createDateTime,omitempty"`
	LastUpdateDate      int64              `json:"lastUpdateDate,omitempty"`
	Quantity            int                `json:"quantity"`
	StockCode           string             `json:"stockCode"`
	StockUnitType       string             `json:"stockUnitType,omitempty"`
	DimensionalWeight   float64            `json:"dimensionalWeight"`
	Description         string             `json:"description"`
	CurrencyType        string             `json:"currencyType"`
	ListPrice           float64            `json:"listPrice"`
	SalePrice           float64            `json:"salePrice"`
	VATRate             int                `json:"vatRate"`
	HasActiveCampaign   bool               `json:"hasActiveCampaign,omitempty"`
	Locked              bool               `json:"locked,omitempty"`
	OnSale              bool               `json:"onSale,omitempty"`
	PlatformListingID   string             `json:"platformListingId,omitempty"`
	ProductCode         int64              `json:"productCode,omitempty"`
	ProductContentID    int64              `json:"productContentId,omitempty"`
	SupplierID          int64              `json:"supplierId,omitempty"`
	ID                  string             `json:"id,omitempty"`
	CargoCompanyID      int                `json:"cargoCompanyId"`
	ShipmentAddressID   int                `json:"shipmentAddressId,omitempty"`
	ReturningAddressID  int                `json:"returningAddressId,omitempty"`
	DeliveryOption      *DeliveryOption    `json:"deliveryOption,omitempty"`
	Images              []ProductImage     `json:"images"`
	Attributes          []ProductAttribute `json:"attributes"`
	Rejected            bool               `json:"rejected,omitempty"`
	RejectReasonDetails []interface{}      `json:"rejectReasonDetails,omitempty"`
	Blacklisted         bool               `json:"blacklisted,omitempty"`
	HasHTMLContent      bool               `json:"hasHtmlContent,omitempty"`
	ProductURL          string             `json:"productUrl,omitempty"`
	DeliveryDuration    int                `json:"deliveryDuration,omitempty"`
}

// ProductImage represents a product image
type ProductImage struct {
	URL string `json:"url"`
}

// ProductAttribute represents a product attribute
type ProductAttribute struct {
	AttributeID          int    `json:"attributeId"`                    // Zorunlu
	AttributeName        string `json:"attributeName,omitempty"`        // Opsiyonel – sadece okuma
	AttributeValue       string `json:"attributeValue,omitempty"`       // Opsiyonel – okuma
	AttributeValueID     int    `json:"attributeValueId,omitempty"`     // Opsiyonel – yazma
	CustomAttributeValue string `json:"customAttributeValue,omitempty"` // Opsiyonel – yazma
}

// DeliveryOption represents delivery options
type DeliveryOption struct {
	DeliveryDuration int    `json:"deliveryDuration"`
	FastDeliveryType string `json:"fastDeliveryType,omitempty"`
}

// CreateProductsRequest represents a request to create products
type CreateProductsRequest struct {
	Items []Product `json:"items"`
}

// UpdateProductsRequest represents a request to update products
type UpdateProductsRequest struct {
	Items []Product `json:"items"`
}

// BatchResponse represents a batch operation response
type BatchResponse struct {
	BatchRequestID string `json:"batchRequestId"`
}

// BatchStatusResponse represents batch status check response
type BatchStatusResponse struct {
	BatchRequestID   string              `json:"batchRequestId"`
	Status           string              `json:"status"`
	CreationDate     int64               `json:"creationDate"`
	LastModification int64               `json:"lastModification"`
	SourceType       string              `json:"sourceType"`
	ItemCount        int                 `json:"itemCount"`
	FailedItemCount  int                 `json:"failedItemCount"`
	BatchRequestType string              `json:"batchRequestType"`
	Items            []BatchResponseItem `json:"items,omitempty"`

	// Legacy fields for backward compatibility
	SucceededItems   int `json:"-"`
	FailedItemsCount int `json:"-"`
}

// BatchResponseItem represents an item in batch response
type BatchResponseItem struct {
	RequestItem    interface{} `json:"requestItem"`
	Status         string      `json:"status"`
	FailureReasons []string    `json:"failureReasons,omitempty"`
}

// BatchFailedItem represents a failed item in batch processing (legacy)
type BatchFailedItem struct {
	Barcode string      `json:"barcode"`
	Errors  []ErrorItem `json:"errors"`
}

// PriceInventoryItem represents a price and inventory update item
type PriceInventoryItem struct {
	Barcode   string  `json:"barcode"`
	Quantity  int     `json:"quantity"`
	SalePrice float64 `json:"salePrice"`
	ListPrice float64 `json:"listPrice"`
}

// ShipmentLine represents a line item in a shipment
type ShipmentLine struct {
	LineID      int64   `json:"lineId"`
	Barcode     string  `json:"barcode"`
	Quantity    int     `json:"quantity"`
	Price       float64 `json:"price"`
	ProductName string  `json:"productName"`
	MerchantSKU string  `json:"merchantSku"`
	PackageID   int64   `json:"packageId"`
}

// Address represents a seller address
type Address struct {
	ID                 int    `json:"id"`
	AddressType        string `json:"addressType"`
	Country            string `json:"country"`
	City               string `json:"city"`
	CityCode           int    `json:"cityCode"`
	District           string `json:"district"`
	DistrictID         int    `json:"districtId"`
	PostCode           string `json:"postCode"`
	Address            string `json:"address"`
	FullAddress        string `json:"fullAddress"`
	IsDefault          bool   `json:"isDefault"`
	IsShipmentAddress  bool   `json:"isShipmentAddress"`
	IsReturningAddress bool   `json:"isReturningAddress"`
	IsInvoiceAddress   bool   `json:"isInvoiceAddress"`
}

// UpdatePackageStatusRequest represents a package status update request
type UpdatePackageStatusRequest struct {
	Status string                    `json:"status"`
	Lines  []UpdatePackageStatusLine `json:"lines"`
	Params map[string]string         `json:"params,omitempty"`
}

// UpdatePackageStatusLine represents a line item in status update
type UpdatePackageStatusLine struct {
	LineID   int64 `json:"lineId"`
	Quantity int   `json:"quantity"`
}

// CancelPackageLine represents a line item to cancel
type CancelPackageLine struct {
	LineID   int64 `json:"lineId"`
	Quantity int   `json:"quantity"`
}

// SplitGroup represents a group of order lines to split
type SplitGroup struct {
	OrderLineIDs []int64 `json:"orderLineIds"`
}

// QuantitySplit represents quantity-based split configuration
type QuantitySplit struct {
	OrderLineID int64 `json:"orderLineId"`
	Quantities  []int `json:"quantities"`
}

// AlternativeDeliveryRequest represents alternative delivery configuration
type AlternativeDeliveryRequest struct {
	IsPhoneNumber bool              `json:"isPhoneNumber"`
	TrackingInfo  string            `json:"trackingInfo"`
	Params        map[string]string `json:"params"`
	BoxQuantity   *int              `json:"boxQuantity,omitempty"`
	Deci          *float64          `json:"deci,omitempty"`
}

// LaborCost represents labor cost for an order line
type LaborCost struct {
	OrderLineID      int64   `json:"orderLineId"`
	LaborCostPerItem float64 `json:"laborCostPerItem"`
}

// TrackingNumberRequest represents tracking number update request
type TrackingNumberRequest struct {
	TrackingNumber string `json:"trackingNumber"`
}

// Claim represents a return/claim
type Claim struct {
	ID               int64       `json:"id"`
	Status           string      `json:"status"`
	CreatedDate      int64       `json:"createdDate"`
	LastModifiedDate int64       `json:"lastModifiedDate"`
	Items            []ClaimItem `json:"items"`
}

// ClaimItem represents an item in a claim
type ClaimItem struct {
	ID         int64  `json:"id"`
	Barcode    string `json:"barcode"`
	Quantity   int    `json:"quantity"`
	ReasonText string `json:"reasonText"`
}

// ClaimReason represents a claim reason
type ClaimReason struct {
	ClaimIssueReasonID int    `json:"claimIssueReasonId"`
	Reason             string `json:"reason"`
}

// Category represents a product category
type Category struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ParentID int    `json:"parentId,omitempty"`
}

// CategoryAttribute represents a category attribute
type CategoryAttribute struct {
	AttributeID      int              `json:"attributeId"`
	AttributeName    string           `json:"attributeName"`
	Required         bool             `json:"required"`
	AllowCustomValue bool             `json:"allowCustomValue"`
	AttributeValues  []AttributeValue `json:"attributeValues,omitempty"`
}

// AttributeValue represents an attribute value option
type AttributeValue struct {
	AttributeValueID int    `json:"attributeValueId"`
	Value            string `json:"value"`
}

// Brand represents a brand
type Brand struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// TestAuthentication tests if the API credentials are valid
func (c *Client) TestAuthentication(ctx context.Context) error {
	// Use products endpoint to test authentication since it's more reliable
	req := &Request{
		Method: http.MethodGet,
		Path:   c.resolve(EndpointGetProductsKey, c.sellerID),
		Query: url.Values{
			"size": []string{"1"},
		},
	}
	return c.Do(ctx, req)
}

// HealthCheck checks the API health status
func (c *Client) HealthCheck(ctx context.Context) error {

	// Since there's no dedicated health endpoint, we use a simple products query
	req := &Request{
		Method: http.MethodGet,
		Path:   c.resolve(EndpointGetProductsKey, c.sellerID),
		Query: url.Values{
			"size": []string{"1"},
			"page": []string{"0"},
		},
	}
	return c.Do(ctx, req)
}

// ProductService defines operations for product management
type ProductService interface {
	Create(ctx context.Context, products []Product) (*BatchResponse, error)
	Update(ctx context.Context, products []Product) (*BatchResponse, error)
	Delete(ctx context.Context, barcodes []string) (*BatchResponse, error)
	GetBatchStatus(ctx context.Context, batchRequestID string) (*BatchStatusResponse, error)
	List(ctx context.Context, page, size int) ([]Product, *PaginatedResponse, error)
	ListWithOptions(ctx context.Context, page, size int, opts *ProductListOptions) ([]Product, *PaginatedResponse, error)
	GetByBarcode(ctx context.Context, barcode string) (*Product, error)
}

// OrderService defines operations for order management
type OrderService interface {
	List(ctx context.Context, opts ListOrdersOptions) ([]Order, *PaginatedResponse, error)
	ListLegacy(ctx context.Context, opts ListOrdersOptions) ([]ShipmentPackage, *PaginatedResponse, error)
	UpdateStatus(ctx context.Context, packageID int64, req UpdatePackageStatusRequest) error
	UpdateTrackingNumber(ctx context.Context, packageID int64, trackingNumber string) error
	SendInvoiceLink(ctx context.Context, packageID int64, invoiceLink string) error
	// New methods
	CancelPackageItems(ctx context.Context, packageID int64, lines []CancelPackageLine) error
	SplitPackage(ctx context.Context, packageID int64, orderLineIDs []int64) error
	MultiSplitPackage(ctx context.Context, packageID int64, splitGroups []SplitGroup) error
	QuantitySplitPackage(ctx context.Context, packageID int64, splits []QuantitySplit) error
	UpdateBoxInfo(ctx context.Context, packageID int64, boxQuantity int, deci float64) error
	AlternativeDelivery(ctx context.Context, packageID int64, req AlternativeDeliveryRequest) error
	ManualDeliver(ctx context.Context, cargoTrackingNumber string) error
	ManualReturn(ctx context.Context, cargoTrackingNumber string) error
	UpdateCargoProvider(ctx context.Context, packageID int64, cargoProvider string) error
	UpdateWarehouse(ctx context.Context, packageID int64, warehouseID int) error
	ExtendDeliveryDate(ctx context.Context, packageID int64, extendedDayCount int) error
	UpdateLaborCosts(ctx context.Context, packageID int64, costs []LaborCost) error
	DeliveredByService(ctx context.Context, packageID int64) error
}

// PriceInventoryService defines operations for price and inventory management
type PriceInventoryService interface {
	Update(ctx context.Context, items []PriceInventoryItem) (*BatchResponse, error)
	DeleteProduct(ctx context.Context, barcode string) error
	DeleteProducts(ctx context.Context, barcodes []string) error
	ApplyPriceIncrease(ctx context.Context, items []PriceInventoryItem, percentage float64) (*BatchResponse, error)
	ApplyPriceDecrease(ctx context.Context, items []PriceInventoryItem, percentage float64) (*BatchResponse, error)
}

// ClaimService defines operations for claim/return management
type ClaimService interface {
	List(ctx context.Context, status string, page, size int) ([]Claim, *PaginatedResponse, error)
	GetReasons(ctx context.Context) ([]ClaimReason, error)
	ApproveItems(ctx context.Context, claimID int64, itemIDs []int64) error
	RejectItems(ctx context.Context, claimID int64, reasonID int, itemIDs []int64, description string) error
}

// AddressService defines operations for address management
type AddressService interface {
	List(ctx context.Context) ([]Address, error)
}

// CategoryService defines operations for category and brand management
type CategoryService interface {
	ListCategories(ctx context.Context) ([]Category, error)
	GetCategoryAttributes(ctx context.Context, categoryID int) ([]CategoryAttribute, error)
	ListBrands(ctx context.Context, page, size int) ([]Brand, *PaginatedResponse, error)
}

// ListOrdersOptions represents options for listing orders
type ListOrdersOptions struct {
	Status           string
	StartDate        *time.Time
	EndDate          *time.Time
	OrderByField     string
	OrderByDirection string
	Page             int
	Size             int
}

type ProductListOptions struct {
	Approved      *bool      `json:"approved,omitempty"`
	Archived      *bool      `json:"archived,omitempty"`
	Barcode       string     `json:"barcode,omitempty"`
	StockCode     string     `json:"stockCode,omitempty"`
	ProductMainID string     `json:"productMainId,omitempty"`
	OnSale        *bool      `json:"onSale,omitempty"`
	Rejected      *bool      `json:"rejected,omitempty"`
	Blacklisted   *bool      `json:"blacklisted,omitempty"`
	StartDate     *time.Time `json:"startDate,omitempty"`
	EndDate       *time.Time `json:"endDate,omitempty"`
	DateQueryType string     `json:"dateQueryType,omitempty"` // CREATED_DATE or LAST_MODIFIED_DATE
	SupplierID    int64      `json:"supplierId,omitempty"`
	BrandIDs      []int      `json:"brandIds,omitempty"`
}

// productService implements ProductService
type productService struct {
	client *Client
}

func (s *productService) Create(ctx context.Context, products []Product) (*BatchResponse, error) {
	req := &Request{
		Method: http.MethodPost,
		Path:   s.client.resolve(EndpointCreateProductsKey, s.client.sellerID),
		Body:   CreateProductsRequest{Items: products},
		Result: &BatchResponse{},
	}
	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	return req.Result.(*BatchResponse), nil
}

func (s *productService) Update(ctx context.Context, products []Product) (*BatchResponse, error) {
	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointUpdateProductsKey, s.client.sellerID),
		Body:   UpdateProductsRequest{Items: products},
		Result: &BatchResponse{},
	}
	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	return req.Result.(*BatchResponse), nil
}

func (s *productService) Delete(ctx context.Context, barcodes []string) (*BatchResponse, error) {
	type deleteItem struct {
		Barcode string `json:"barcode"`
	}

	items := make([]deleteItem, len(barcodes))
	for i, barcode := range barcodes {
		items[i] = deleteItem{Barcode: barcode}
	}

	body := map[string]interface{}{
		"items": items,
	}

	req := &Request{
		Method: http.MethodDelete,
		Path:   s.client.resolve(EndpointDeleteProductsKey, s.client.sellerID),
		Body:   body,
		Result: &BatchResponse{},
	}
	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	return req.Result.(*BatchResponse), nil
}

func (s *productService) GetBatchStatus(ctx context.Context, batchRequestID string) (*BatchStatusResponse, error) {
	// Debug için raw response'u da alalım
	var rawResp []byte
	req := &Request{
		Method:      http.MethodGet,
		Path:        s.client.resolve(EndpointGetBatchRequestResultKey, s.client.sellerID, batchRequestID),
		Result:      &rawResp,
		RawResponse: true,
	}
	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse response
	var result BatchStatusResponse
	if err := json.Unmarshal(rawResp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse batch status response: %w", err)
	}

	// Calculate legacy fields for backward compatibility
	// Safe calculations that won't break if API response format changes
	if result.FailedItemCount >= 0 {
		result.FailedItemsCount = result.FailedItemCount
	}
	if result.ItemCount >= 0 && result.FailedItemCount >= 0 {
		result.SucceededItems = result.ItemCount - result.FailedItemCount
		if result.SucceededItems < 0 {
			result.SucceededItems = 0 // Prevent negative values
		}
	}

	return &result, nil
}

func (s *productService) List(ctx context.Context, page, size int) ([]Product, *PaginatedResponse, error) {
	return s.ListWithOptions(ctx, page, size, nil)
}

func (s *productService) ListWithOptions(ctx context.Context, page, size int, opts *ProductListOptions) ([]Product, *PaginatedResponse, error) {
	type response struct {
		Content []Product `json:"content"`
		PaginatedResponse
	}

	query := url.Values{
		"page": []string{strconv.Itoa(page)},
		"size": []string{strconv.Itoa(size)},
	}

	// Add optional parameters if provided
	if opts != nil {
		if opts.Approved != nil {
			query.Set("approved", strconv.FormatBool(*opts.Approved))
		}
		if opts.Archived != nil {
			query.Set("archived", strconv.FormatBool(*opts.Archived))
		}
		if opts.Barcode != "" {
			query.Set("barcode", opts.Barcode)
		}
		if opts.StockCode != "" {
			query.Set("stockCode", opts.StockCode)
		}
		if opts.ProductMainID != "" {
			query.Set("productMainId", opts.ProductMainID)
		}
		if opts.OnSale != nil {
			query.Set("onSale", strconv.FormatBool(*opts.OnSale))
		}
		if opts.Rejected != nil {
			query.Set("rejected", strconv.FormatBool(*opts.Rejected))
		}
		if opts.Blacklisted != nil {
			query.Set("blacklisted", strconv.FormatBool(*opts.Blacklisted))
		}
		if opts.StartDate != nil {
			query.Set("startDate", strconv.FormatInt(opts.StartDate.UnixMilli(), 10))
		}
		if opts.EndDate != nil {
			query.Set("endDate", strconv.FormatInt(opts.EndDate.UnixMilli(), 10))
		}
		if opts.DateQueryType != "" {
			query.Set("dateQueryType", opts.DateQueryType)
		}
		if opts.SupplierID != 0 {
			query.Set("supplierId", strconv.FormatInt(opts.SupplierID, 10))
		}
		if len(opts.BrandIDs) > 0 {
			for _, brandID := range opts.BrandIDs {
				query.Add("brandIds", strconv.Itoa(brandID))
			}
		}
	}

	result := &response{}
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetProductsKey, s.client.sellerID),
		Query:  query,
		Result: result,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	return result.Content, &result.PaginatedResponse, nil
}

func (s *productService) GetByBarcode(ctx context.Context, barcode string) (*Product, error) {
	type response struct {
		Content []Product `json:"content"`
	}

	result := &response{}
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetProductsKey, s.client.sellerID),
		Query: url.Values{
			"barcode": []string{barcode},
		},
		Result: result,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("product not found with barcode: %s", barcode)
	}

	return &result.Content[0], nil
}

// orderService implements OrderService
type orderService struct {
	client *Client
}

func (s *orderService) List(ctx context.Context, opts ListOrdersOptions) ([]Order, *PaginatedResponse, error) {
	type response struct {
		Content []Order `json:"content"`
		PaginatedResponse
	}

	query := url.Values{
		"page": []string{strconv.Itoa(opts.Page)},
		"size": []string{strconv.Itoa(opts.Size)},
	}

	if opts.Status != "" {
		query.Set("status", opts.Status)
	}
	if opts.StartDate != nil {
		query.Set("startDate", strconv.FormatInt(opts.StartDate.UnixMilli(), 10))
	}
	if opts.EndDate != nil {
		query.Set("endDate", strconv.FormatInt(opts.EndDate.UnixMilli(), 10))
	}
	if opts.OrderByField != "" {
		query.Set("orderByField", opts.OrderByField)
	}
	if opts.OrderByDirection != "" {
		query.Set("orderByDirection", opts.OrderByDirection)
	}

	result := &response{}
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetOrdersKey, s.client.sellerID),
		Query:  query,
		Result: result,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	return result.Content, &result.PaginatedResponse, nil
}

func (s *orderService) ListLegacy(ctx context.Context, opts ListOrdersOptions) ([]ShipmentPackage, *PaginatedResponse, error) {
	type response struct {
		Content []ShipmentPackage `json:"content"`
		PaginatedResponse
	}

	query := url.Values{
		"page": []string{strconv.Itoa(opts.Page)},
		"size": []string{strconv.Itoa(opts.Size)},
	}

	if opts.Status != "" {
		query.Set("status", opts.Status)
	}
	if opts.StartDate != nil {
		query.Set("startDate", strconv.FormatInt(opts.StartDate.UnixMilli(), 10))
	}
	if opts.EndDate != nil {
		query.Set("endDate", strconv.FormatInt(opts.EndDate.UnixMilli(), 10))
	}
	if opts.OrderByField != "" {
		query.Set("orderByField", opts.OrderByField)
	}
	if opts.OrderByDirection != "" {
		query.Set("orderByDirection", opts.OrderByDirection)
	}

	result := &response{}
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetOrdersKey, s.client.sellerID),
		Query:  query,
		Result: result,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	return result.Content, &result.PaginatedResponse, nil
}

func (s *orderService) UpdateStatus(ctx context.Context, packageID int64, req UpdatePackageStatusRequest) error {
	request := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointUpdatePackageStatusKey, s.client.sellerID, packageID),
		Body:   req,
	}
	return s.client.Do(ctx, request)
}

func (s *orderService) UpdateTrackingNumber(ctx context.Context, packageID int64, trackingNumber string) error {
	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointUpdateTrackingNumberKey, s.client.sellerID, packageID),
		Body:   TrackingNumberRequest{TrackingNumber: trackingNumber},
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) SendInvoiceLink(ctx context.Context, packageID int64, invoiceLink string) error {
	req := &Request{
		Method: http.MethodPost,
		Path:   s.client.resolve(EndpointSendInvoiceLinkKey, s.client.sellerID),
		Body:   InvoiceLinkRequest{ShipmentPackageID: packageID, InvoiceLink: invoiceLink},
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) CancelPackageItems(ctx context.Context, packageID int64, lines []CancelPackageLine) error {
	body := map[string]interface{}{
		"lines":    lines,
		"reasonId": 0, // TODO: Make this configurable
	}

	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointCancelPackageItemsKey, s.client.sellerID, packageID),
		Body:   body,
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) SplitPackage(ctx context.Context, packageID int64, orderLineIDs []int64) error {
	body := map[string]interface{}{
		"orderLineIds": orderLineIDs,
	}

	req := &Request{
		Method: http.MethodPost,
		Path:   s.client.resolve(EndpointSplitPackageKey, s.client.sellerID, packageID),
		Body:   body,
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) MultiSplitPackage(ctx context.Context, packageID int64, splitGroups []SplitGroup) error {
	body := map[string]interface{}{
		"splitGroups": splitGroups,
	}

	req := &Request{
		Method: http.MethodPost,
		Path:   s.client.resolve(EndpointMultiSplitPackageKey, s.client.sellerID, packageID),
		Body:   body,
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) QuantitySplitPackage(ctx context.Context, packageID int64, splits []QuantitySplit) error {
	body := map[string]interface{}{
		"quantitySplit": splits,
	}

	req := &Request{
		Method: http.MethodPost,
		Path:   s.client.resolve(EndpointQuantitySplitPackageKey, s.client.sellerID, packageID),
		Body:   body,
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) UpdateBoxInfo(ctx context.Context, packageID int64, boxQuantity int, deci float64) error {
	body := map[string]interface{}{
		"boxQuantity": boxQuantity,
		"deci":        deci,
	}

	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointUpdateBoxInfoKey, s.client.sellerID, packageID),
		Body:   body,
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) AlternativeDelivery(ctx context.Context, packageID int64, req AlternativeDeliveryRequest) error {
	request := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointAlternativeDeliveryKey, s.client.sellerID, packageID),
		Body:   req,
	}
	return s.client.Do(ctx, request)
}

func (s *orderService) ManualDeliver(ctx context.Context, cargoTrackingNumber string) error {
	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointManualDeliverKey, s.client.sellerID, cargoTrackingNumber),
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) ManualReturn(ctx context.Context, cargoTrackingNumber string) error {
	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointManualReturnKey, s.client.sellerID, cargoTrackingNumber),
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) UpdateCargoProvider(ctx context.Context, packageID int64, cargoProvider string) error {
	body := map[string]interface{}{
		"cargoProvider": cargoProvider,
	}

	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointUpdateCargoProviderKey, s.client.sellerID, packageID),
		Body:   body,
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) UpdateWarehouse(ctx context.Context, packageID int64, warehouseID int) error {
	body := map[string]interface{}{
		"warehouseId": warehouseID,
	}

	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointUpdateWarehouseKey, s.client.sellerID, packageID),
		Body:   body,
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) ExtendDeliveryDate(ctx context.Context, packageID int64, extendedDayCount int) error {
	body := map[string]interface{}{
		"extendedDayCount": extendedDayCount,
	}

	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointExtendDeliveryDateKey, s.client.sellerID, packageID),
		Body:   body,
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) UpdateLaborCosts(ctx context.Context, packageID int64, costs []LaborCost) error {
	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointUpdateLaborCostsKey, s.client.sellerID, packageID),
		Body:   costs,
	}
	return s.client.Do(ctx, req)
}

func (s *orderService) DeliveredByService(ctx context.Context, packageID int64) error {
	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointDeliveredByServiceKey, s.client.sellerID, packageID),
	}
	return s.client.Do(ctx, req)
}

// priceInventoryService implements PriceInventoryService
type priceInventoryService struct {
	client *Client
}

func (s *priceInventoryService) Update(ctx context.Context, items []PriceInventoryItem) (*BatchResponse, error) {
	req := &Request{
		Method: http.MethodPost,
		Path:   s.client.resolve(EndpointUpdatePriceInventoryKey, s.client.sellerID),
		Body:   map[string]interface{}{"items": items},
		Result: &BatchResponse{},
	}
	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	return req.Result.(*BatchResponse), nil
}

func (s *priceInventoryService) DeleteProduct(ctx context.Context, barcode string) error {
	items := []PriceInventoryItem{
		{
			Barcode:   barcode,
			Quantity:  0,
			SalePrice: 0,
			ListPrice: 0,
		},
	}
	_, err := s.Update(ctx, items)
	return err
}

func (s *priceInventoryService) DeleteProducts(ctx context.Context, barcodes []string) error {
	items := make([]PriceInventoryItem, len(barcodes))
	for i, barcode := range barcodes {
		items[i] = PriceInventoryItem{
			Barcode:   barcode,
			Quantity:  0,
			SalePrice: 0,
			ListPrice: 0,
		}
	}
	_, err := s.Update(ctx, items)
	return err
}

func (s *priceInventoryService) ApplyPriceIncrease(ctx context.Context, items []PriceInventoryItem, percentage float64) (*BatchResponse, error) {
	// Create a copy to avoid modifying the original slice
	updatedItems := make([]PriceInventoryItem, len(items))
	for i, item := range items {
		updatedItems[i] = item
		updatedItems[i].SalePrice = item.SalePrice * (1 + percentage/100)
		updatedItems[i].ListPrice = item.ListPrice * (1 + percentage/100)
	}
	return s.Update(ctx, updatedItems)
}

func (s *priceInventoryService) ApplyPriceDecrease(ctx context.Context, items []PriceInventoryItem, percentage float64) (*BatchResponse, error) {
	// Create a copy to avoid modifying the original slice
	updatedItems := make([]PriceInventoryItem, len(items))
	for i, item := range items {
		updatedItems[i] = item
		updatedItems[i].SalePrice = item.SalePrice * (1 - percentage/100)
		updatedItems[i].ListPrice = item.ListPrice * (1 - percentage/100)
	}
	return s.Update(ctx, updatedItems)
}

// claimService implements ClaimService
type claimService struct {
	client *Client
}

func (s *claimService) List(ctx context.Context, status string, page, size int) ([]Claim, *PaginatedResponse, error) {
	type response struct {
		Content []Claim `json:"content"`
		PaginatedResponse
	}

	query := url.Values{
		"page": []string{strconv.Itoa(page)},
		"size": []string{strconv.Itoa(size)},
	}
	if status != "" {
		query.Set("claimItemStatus", status)
	}

	result := &response{}
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetClaimsKey, s.client.sellerID),
		Query:  query,
		Result: result,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	return result.Content, &result.PaginatedResponse, nil
}

func (s *claimService) GetReasons(ctx context.Context) ([]ClaimReason, error) {
	var reasons []ClaimReason
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetClaimIssueReasonsKey),
		Result: &reasons,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	return reasons, nil
}

func (s *claimService) ApproveItems(ctx context.Context, claimID int64, itemIDs []int64) error {
	body := map[string]interface{}{
		"claimLineItemIdList": itemIDs,
		"params":              map[string]string{},
	}

	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointApproveClaimKey, s.client.sellerID, strconv.FormatInt(claimID, 10)),
		Body:   body,
	}

	return s.client.Do(ctx, req)
}

func (s *claimService) RejectItems(ctx context.Context, claimID int64, reasonID int, itemIDs []int64, description string) error {
	// Convert to string IDs
	stringIDs := make([]string, len(itemIDs))
	for i, id := range itemIDs {
		stringIDs[i] = strconv.FormatInt(id, 10)
	}

	// Build query parameters
	query := url.Values{
		"claimIssueReasonId": []string{strconv.Itoa(reasonID)},
		"description":        []string{description},
	}
	for _, id := range stringIDs {
		query.Add("claimItemIdList", id)
	}

	req := &Request{
		Method: http.MethodPost,
		Path:   s.client.resolve(EndpointRejectClaimKey, s.client.sellerID, strconv.FormatInt(claimID, 10)),
		Query:  query,
	}

	return s.client.Do(ctx, req)
}

// addressService implements AddressService
type addressService struct {
	client *Client
}

func (s *addressService) List(ctx context.Context) ([]Address, error) {
	type response struct {
		SupplierAddresses       []Address `json:"supplierAddresses"`
		DefaultShipmentAddress  *Address  `json:"defaultShipmentAddress,omitempty"`
		DefaultInvoiceAddress   *Address  `json:"defaultInvoiceAddress,omitempty"`
		DefaultReturningAddress struct {
			Present bool `json:"present"`
		} `json:"defaultReturningAddress,omitempty"`
	}

	result := &response{}
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointSellerAddressesKey, s.client.sellerID),
		Result: result,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	return result.SupplierAddresses, nil
}

// categoryService implements CategoryService
type categoryService struct {
	client *Client
}

func (s *categoryService) ListCategories(ctx context.Context) ([]Category, error) {
	type response struct {
		Categories []Category `json:"categories"`
	}

	result := &response{}
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetCategoriesKey),
		Result: result,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	return result.Categories, nil
}

func (s *categoryService) GetCategoryAttributes(ctx context.Context, categoryID int) ([]CategoryAttribute, error) {
	// Trendyol API response formatı farklı, parse edelim
	type attrResponse struct {
		ID                 int    `json:"id"`
		Name               string `json:"name"`
		DisplayName        string `json:"displayName"`
		CategoryAttributes []struct {
			Attribute struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"attribute"`
			AttributeValues []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"attributeValues"`
			Required    bool `json:"required"`
			AllowCustom bool `json:"allowCustom"`
			Varianter   bool `json:"varianter"`
			Slicer      bool `json:"slicer"`
		} `json:"categoryAttributes"`
	}

	var response attrResponse
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetCategoryAttributesKey, categoryID),
		Result: &response,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert to our CategoryAttribute format
	attributes := make([]CategoryAttribute, len(response.CategoryAttributes))
	for i, catAttr := range response.CategoryAttributes {
		attributes[i] = CategoryAttribute{
			AttributeID:      catAttr.Attribute.ID,
			AttributeName:    catAttr.Attribute.Name,
			Required:         catAttr.Required,
			AllowCustomValue: catAttr.AllowCustom,
		}

		// Convert attribute values
		if len(catAttr.AttributeValues) > 0 {
			attributes[i].AttributeValues = make([]AttributeValue, len(catAttr.AttributeValues))
			for j, val := range catAttr.AttributeValues {
				attributes[i].AttributeValues[j] = AttributeValue{
					AttributeValueID: val.ID,
					Value:            val.Name,
				}
			}
		}
	}

	return attributes, nil
}

func (s *categoryService) ListBrands(ctx context.Context, page, size int) ([]Brand, *PaginatedResponse, error) {
	type response struct {
		Brands []Brand `json:"brands"`
	}

	result := &response{}
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetBrandsKey),
		Query: url.Values{
			"page": []string{strconv.Itoa(page)},
			"size": []string{strconv.Itoa(size)},
		},
		Result: result,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	// Brands endpoint doesn't return pagination info, so we create a dummy one
	pagination := &PaginatedResponse{
		Page:         page,
		Size:         len(result.Brands),
		TotalPages:   -1, // Unknown
		TotalElement: -1, // Unknown
	}

	return result.Brands, pagination, nil
}

// InvoiceLinkRequest represents an invoice link submission
type InvoiceLinkRequest struct {
	ShipmentPackageID int64  `json:"shipmentPackageId"`
	InvoiceLink       string `json:"invoiceLink"`
}

// UpdatePriceInventoryRequest represents a price and inventory update request
type UpdatePriceInventoryRequest struct {
	Items []PriceInventoryItem `json:"items"`
}

// GetProductsResponse represents the response from listing products
type GetProductsResponse struct {
	Content []Product `json:"content"`
	PaginatedResponse
}

// GetShipmentPackagesResponse represents the response from listing orders
type GetShipmentPackagesResponse struct {
	ShipmentPackages []ShipmentPackage `json:"shipmentPackages"`
	PaginatedResponse
}

// ShipmentProviderService defines operations for shipment provider management
type ShipmentProviderService interface {
	List(ctx context.Context) ([]ShipmentProvider, error)
}

// ShipmentProvider represents a cargo/shipment company
type ShipmentProvider struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// shipmentProviderService implements ShipmentProviderService
type shipmentProviderService struct {
	client *Client
}

func (s *shipmentProviderService) List(ctx context.Context) ([]ShipmentProvider, error) {
	var providers []ShipmentProvider
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetShipmentProvidersKey),
		Result: &providers,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	return providers, nil
}

// GetSellerID returns the configured seller ID
func (c *Client) GetSellerID() string {
	return c.sellerID
}

// SetBaseURL allows overriding the API base URL
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// GetBaseURL returns the current API base URL
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// WithContext is a helper method to create a context with timeout
func (c *Client) WithContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// Close stops the rate limiter
func (c *Client) Close() {
	if c.rateLimiter != nil && c.rateLimiter.ticker != nil {
		c.rateLimiter.ticker.Stop()
	}
}

// FinanceService provides finance and accounting operations
type FinanceService interface {
	GetSettlements(ctx context.Context, startDate, endDate time.Time, page, size int) ([]Settlement, *PaginatedResponse, error)
	GetCargoInvoiceDetails(ctx context.Context, invoiceSerialNumber string) ([]CargoInvoiceDetail, error)
}

// Settlement represents a financial settlement record
type Settlement struct {
	SettlementDate      int64   `json:"settlementDate"`
	PaymentDate         int64   `json:"paymentDate"`
	TransactionType     string  `json:"transactionType"`
	OrderNumber         string  `json:"orderNumber"`
	Description         string  `json:"description"`
	Amount              float64 `json:"amount"`
	CommissionAmount    float64 `json:"commissionAmount"`
	SellerRevenue       float64 `json:"sellerRevenue"`
	InvoiceSerialNumber string  `json:"invoiceSerialNumber,omitempty"`
}

// CargoInvoiceDetail represents cargo invoice detail
type CargoInvoiceDetail struct {
	OrderNumber       string  `json:"orderNumber"`
	ShipmentPackageID int64   `json:"shipmentPackageId"`
	CargoAmount       float64 `json:"cargoAmount"`
	CargoProviderName string  `json:"cargoProviderName"`
}

// CommonLabelService provides common label/barcode operations
type CommonLabelService interface {
	CreateLabel(ctx context.Context, cargoTrackingNumber string, req CommonLabelRequest) error
	GetLabel(ctx context.Context, cargoTrackingNumber string) ([]byte, error)
}

// CommonLabelRequest represents a common label creation request
type CommonLabelRequest struct {
	Format           string  `json:"format"` // e.g., "ZPL"
	BoxQuantity      int     `json:"boxQuantity"`
	VolumetricHeight float64 `json:"volumetricHeight,omitempty"`
}

// MemberService provides member/location operations
type MemberService interface {
	GetCountries(ctx context.Context) ([]Country, error)
	GetCountryCities(ctx context.Context, countryCode string) ([]City, error)
	GetDomesticCities(ctx context.Context, countryCode string) ([]City, error)
}

// Country represents a country
type Country struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// City represents a city
type City struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CountryID int    `json:"countryId"`
	Code      string `json:"code,omitempty"`
}

// TestService provides test environment operations
type TestService interface {
	CreateTestOrder(ctx context.Context, req TestOrderRequest) (*TestOrderResponse, error)
	UpdateTestOrderStatus(ctx context.Context, packageID int64, req UpdatePackageStatusRequest) error
	SetClaimWaitingInAction(ctx context.Context, shipmentPackageID int64) error
}

// TestOrderRequest represents a test order creation request
type TestOrderRequest struct {
	Customer        TestCustomer    `json:"customer"`
	InvoiceAddress  TestAddress     `json:"invoiceAddress"`
	ShippingAddress TestAddress     `json:"shippingAddress"`
	Lines           []TestOrderLine `json:"lines"`
	Seller          TestSeller      `json:"seller"`
	Commercial      bool            `json:"commercial"`
	MicroRegion     string          `json:"microRegion,omitempty"`
}

// TestCustomer represents test order customer
type TestCustomer struct {
	CustomerFirstName string `json:"customerFirstName"`
	CustomerLastName  string `json:"customerLastName"`
}

// TestAddress represents test order address
type TestAddress struct {
	AddressText       string `json:"addressText"`
	City              string `json:"city"`
	Company           string `json:"company,omitempty"`
	District          string `json:"district"`
	InvoiceFirstName  string `json:"invoiceFirstName,omitempty"`
	InvoiceLastName   string `json:"invoiceLastName,omitempty"`
	ShippingFirstName string `json:"shippingFirstName,omitempty"`
	ShippingLastName  string `json:"shippingLastName,omitempty"`
	Latitude          string `json:"latitude,omitempty"`
	Longitude         string `json:"longitude,omitempty"`
	Neighborhood      string `json:"neighborhood,omitempty"`
	Phone             string `json:"phone"`
	PostalCode        string `json:"postalCode,omitempty"`
	Email             string `json:"email"`
	InvoiceTaxNumber  string `json:"invoiceTaxNumber,omitempty"`
	InvoiceTaxOffice  string `json:"invoiceTaxOffice,omitempty"`
}

// TestOrderLine represents test order line item
type TestOrderLine struct {
	Barcode            string  `json:"barcode"`
	Quantity           int     `json:"quantity"`
	DiscountPercentage float64 `json:"discountPercentage,omitempty"`
}

// TestSeller represents test order seller info
type TestSeller struct {
	SellerID int `json:"sellerId"`
}

// TestOrderResponse represents test order creation response
type TestOrderResponse struct {
	OrderNumber       string `json:"orderNumber"`
	ShipmentPackageID int64  `json:"shipmentPackageId"`
}

// financeService implements FinanceService
type financeService struct {
	client *Client
}

func (s *financeService) GetSettlements(ctx context.Context, startDate, endDate time.Time, page, size int) ([]Settlement, *PaginatedResponse, error) {
	type response struct {
		Content []Settlement `json:"content"`
		PaginatedResponse
	}

	result := &response{}
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetSettlementsKey, s.client.sellerID),
		Query: url.Values{
			"startDate": []string{strconv.FormatInt(startDate.UnixMilli(), 10)},
			"endDate":   []string{strconv.FormatInt(endDate.UnixMilli(), 10)},
			"page":      []string{strconv.Itoa(page)},
			"size":      []string{strconv.Itoa(size)},
		},
		Result: result,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	return result.Content, &result.PaginatedResponse, nil
}

func (s *financeService) GetCargoInvoiceDetails(ctx context.Context, invoiceSerialNumber string) ([]CargoInvoiceDetail, error) {
	var details []CargoInvoiceDetail
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetCargoInvoiceDetailsKey, s.client.sellerID, invoiceSerialNumber),
		Result: &details,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	return details, nil
}

// commonLabelService implements CommonLabelService
type commonLabelService struct {
	client *Client
}

func (s *commonLabelService) CreateLabel(ctx context.Context, cargoTrackingNumber string, req CommonLabelRequest) error {
	request := &Request{
		Method: http.MethodPost,
		Path:   s.client.resolve(EndpointCreateCommonLabelKey, s.client.sellerID, cargoTrackingNumber),
		Body:   req,
	}

	return s.client.Do(ctx, request)
}

func (s *commonLabelService) GetLabel(ctx context.Context, cargoTrackingNumber string) ([]byte, error) {
	var result []byte
	req := &Request{
		Method:      http.MethodGet,
		Path:        s.client.resolve(EndpointGetCommonLabelKey, s.client.sellerID, cargoTrackingNumber),
		Result:      &result,
		RawResponse: true,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// memberService implements MemberService
type memberService struct {
	client *Client
}

func (s *memberService) GetCountries(ctx context.Context) ([]Country, error) {
	var countries []Country
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetCountriesKey),
		Result: &countries,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	return countries, nil
}

func (s *memberService) GetCountryCities(ctx context.Context, countryCode string) ([]City, error) {
	var cities []City
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetCountryCitiesKey, countryCode),
		Result: &cities,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	return cities, nil
}

func (s *memberService) GetDomesticCities(ctx context.Context, countryCode string) ([]City, error) {
	var cities []City
	req := &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetDomesticCitiesKey, countryCode),
		Result: &cities,
	}

	err := s.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	return cities, nil
}

// testService implements TestService
type testService struct {
	client *Client
}

func (s *testService) CreateTestOrder(ctx context.Context, req TestOrderRequest) (*TestOrderResponse, error) {
	result := &TestOrderResponse{}
	request := &Request{
		Method: http.MethodPost,
		Path:   s.client.resolve(EndpointCreateTestOrderKey),
		Body:   req,
		Result: result,
	}

	err := s.client.Do(ctx, request)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *testService) UpdateTestOrderStatus(ctx context.Context, packageID int64, req UpdatePackageStatusRequest) error {
	request := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointUpdateTestOrderStatusKey, s.client.sellerID, packageID),
		Body:   req,
	}

	return s.client.Do(ctx, request)
}

func (s *testService) SetClaimWaitingInAction(ctx context.Context, shipmentPackageID int64) error {
	body := map[string]interface{}{
		"shipmentPackageId": shipmentPackageID,
	}

	req := &Request{
		Method: http.MethodPut,
		Path:   s.client.resolve(EndpointTestClaimWaitingInActionKey, s.client.sellerID),
		Body:   body,
	}

	return s.client.Do(ctx, req)
}

// Order represents a single order from the new API structure
type Order struct {
	ID                               int64            `json:"id"` // shipmentPackageId
	ShipmentAddress                  *OrderAddress    `json:"shipmentAddress,omitempty"`
	InvoiceAddress                   *OrderAddress    `json:"invoiceAddress,omitempty"`
	OrderNumber                      string           `json:"orderNumber"`
	GrossAmount                      float64          `json:"grossAmount"`
	TotalDiscount                    float64          `json:"totalDiscount"`
	TotalTyDiscount                  float64          `json:"totalTyDiscount"`
	TaxNumber                        *string          `json:"taxNumber,omitempty"`
	CustomerFirstName                string           `json:"customerFirstName"`
	CustomerLastName                 string           `json:"customerLastName"`
	CustomerEmail                    string           `json:"customerEmail"`
	CustomerID                       int64            `json:"customerId"`
	CargoTrackingNumber              int64            `json:"cargoTrackingNumber,omitempty"`
	CargoTrackingLink                string           `json:"cargoTrackingLink,omitempty"`
	CargoSenderNumber                string           `json:"cargoSenderNumber,omitempty"`
	CargoProviderName                string           `json:"cargoProviderName,omitempty"`
	Lines                            []OrderLine      `json:"lines"`
	OrderDate                        int64            `json:"orderDate"`
	IdentityNumber                   string           `json:"identityNumber"`
	CurrencyCode                     string           `json:"currencyCode"`
	PackageHistories                 []PackageHistory `json:"packageHistories"`
	ShipmentPackageStatus            string           `json:"shipmentPackageStatus"`
	Status                           string           `json:"status"`
	DeliveryType                     string           `json:"deliveryType"`
	TimeSlotID                       int              `json:"timeSlotId"`
	ScheduledDeliveryStoreID         string           `json:"scheduledDeliveryStoreId"`
	EstimatedDeliveryStartDate       int64            `json:"estimatedDeliveryStartDate"`
	EstimatedDeliveryEndDate         int64            `json:"estimatedDeliveryEndDate"`
	TotalPrice                       float64          `json:"totalPrice"`
	DeliveryAddressType              string           `json:"deliveryAddressType"`
	AgreedDeliveryDate               int64            `json:"agreedDeliveryDate"`
	FastDelivery                     bool             `json:"fastDelivery"`
	OriginShipmentDate               int64            `json:"originShipmentDate"`
	LastModifiedDate                 int64            `json:"lastModifiedDate"`
	Commercial                       bool             `json:"commercial"`
	FastDeliveryType                 string           `json:"fastDeliveryType"`
	DeliveredByService               bool             `json:"deliveredByService"`
	AgreedDeliveryDateExtendible     bool             `json:"agreedDeliveryDateExtendible"`
	ExtendedAgreedDeliveryDate       int64            `json:"extendedAgreedDeliveryDate"`
	AgreedDeliveryExtensionEndDate   int64            `json:"agreedDeliveryExtensionEndDate"`
	AgreedDeliveryExtensionStartDate int64            `json:"agreedDeliveryExtensionStartDate"`
	WarehouseID                      int              `json:"warehouseId"`
	GroupDeal                        bool             `json:"groupDeal"`
	Micro                            bool             `json:"micro"`
	GiftBoxRequested                 bool             `json:"giftBoxRequested"`
	EtgbNo                           string           `json:"etgbNo"`
	EtgbDate                         string           `json:"etgbDate"`
	ThreePbyTrendyol                 bool             `json:"3PbyTrendyol"`
	ContainsDangerousProduct         bool             `json:"containsDangerousProduct"`
	CargoDeci                        float64          `json:"cargoDeci"`
	IsCod                            bool             `json:"isCod"`
	CreatedBy                        string           `json:"createdBy"`
	OriginPackageIDs                 []int64          `json:"originPackageIds,omitempty"`
}

// OrderAddress represents detailed address information in orders
type OrderAddress struct {
	ID             int64         `json:"id"`
	FirstName      string        `json:"firstName"`
	LastName       string        `json:"lastName"`
	Company        string        `json:"company"`
	Address1       string        `json:"address1"`
	Address2       string        `json:"address2"`
	AddressLines   *AddressLines `json:"addressLines,omitempty"`
	City           string        `json:"city"`
	CityCode       int           `json:"cityCode"`
	District       string        `json:"district"`
	DistrictID     int           `json:"districtId"`
	CountyID       int           `json:"countyId"`
	CountyName     string        `json:"countyName"`
	ShortAddress   string        `json:"shortAddress"`
	StateName      string        `json:"stateName"`
	PostalCode     string        `json:"postalCode"`
	CountryCode    string        `json:"countryCode"`
	NeighborhoodID int           `json:"neighborhoodId"`
	Neighborhood   string        `json:"neighborhood"`
	Phone          int64         `json:"phone"`
	Latitude       string        `json:"latitude"`
	Longitude      string        `json:"longitude"`
	FullAddress    string        `json:"fullAddress"`
	FullName       string        `json:"fullName"`
}

// AddressLines represents additional address line information
type AddressLines struct {
	AddressLine1 string `json:"addressLine1"`
	AddressLine2 string `json:"addressLine2"`
}

// OrderLine represents a single line item in an order
type OrderLine struct {
	ID                      int64            `json:"id"` // orderLineId
	Quantity                int              `json:"quantity"`
	SalesCampaignID         int              `json:"salesCampaignId"`
	ProductSize             string           `json:"productSize"`
	MerchantSKU             string           `json:"merchantSku"` // stockCode
	ProductName             string           `json:"productName"`
	ProductCode             int              `json:"productCode"` // variantId
	ProductOrigin           string           `json:"productOrigin"`
	MerchantID              int              `json:"merchantId"` // sellerId
	Amount                  float64          `json:"amount"`
	Discount                float64          `json:"discount"`
	TyDiscount              float64          `json:"tyDiscount"`
	DiscountDetails         []DiscountDetail `json:"discountDetails"`
	CurrencyCode            string           `json:"currencyCode"`
	ProductColor            string           `json:"productColor"`
	SKU                     string           `json:"sku"`
	VATBaseAmount           float64          `json:"vatBaseAmount"` // vatRate
	Barcode                 string           `json:"barcode"`
	OrderLineItemStatusName string           `json:"orderLineItemStatusName"`
	Price                   float64          `json:"price"`
	FastDeliveryOptions     []interface{}    `json:"fastDeliveryOptions"`
	ProductCategoryID       int              `json:"productCategoryId"`
}

// DiscountDetail represents discount details for a line item
type DiscountDetail struct {
	LineItemPrice      float64 `json:"lineItemPrice"`
	LineItemDiscount   float64 `json:"lineItemDiscount"`
	LineItemTyDiscount float64 `json:"lineItemTyDiscount"`
}

// PackageHistory represents the status history of a package
type PackageHistory struct {
	CreatedDate int64  `json:"createdDate"`
	Status      string `json:"status"`
}

// ShipmentPackage represents a shipment package in the old API structure
// Keeping for backward compatibility
type ShipmentPackage struct {
	ID                  int64          `json:"id"`
	SupplierID          int            `json:"supplierId"`
	Status              string         `json:"status"`
	CreationDate        int64          `json:"creationDate"`
	LastModifiedDate    int64          `json:"lastModifiedDate"`
	BuyerID             int64          `json:"buyerId"`
	ShippingAddress     *Address       `json:"shippingAddress,omitempty"`
	BillingAddress      *Address       `json:"billingAddress,omitempty"`
	CargoTrackingNumber string         `json:"cargoTrackingNumber,omitempty"`
	CargoProviderName   string         `json:"cargoProviderName,omitempty"`
	Lines               []ShipmentLine `json:"lines"`
}

// resolve returns formatted endpoint path taking overrides into account
func (c *Client) resolve(key string, args ...interface{}) string {
	tmpl, ok := defaultEndpoints[key]
	if c.endpoints != nil {
		if v, ok2 := c.endpoints[key]; ok2 {
			tmpl = v
			ok = true
		}
	}
	if !ok {
		tmpl = key // fallback: use key itself
	}
	if len(args) > 0 {
		return fmt.Sprintf(tmpl, args...)
	}
	return tmpl
}

// WithEndpointOverrides allows overriding specific endpoint templates
func WithEndpointOverrides(m map[string]string) ClientOption {
	return func(c *Client) {
		if c.endpoints == nil {
			c.endpoints = map[string]string{}
		}
		for k, v := range m {
			c.endpoints[k] = v
		}
	}
}

// GetEndpoints, tüm varsayılan uç noktalar ile varsa override edilmiş
// değerlerin **birleşimini** döner. Harita kopyalanarak döndürüldüğü için
// dışarıdan değiştirme client iç durumunu etkilemez.
func (c *Client) GetEndpoints() map[string]string {
	merged := make(map[string]string, len(defaultEndpoints))
	for k, v := range defaultEndpoints {
		merged[k] = v
	}
	if c.endpoints != nil {
		for k, v := range c.endpoints {
			merged[k] = v
		}
	}
	return merged
}
