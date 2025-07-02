package trendyol

// API Endpoints - Product Module
const (
	EndpointGetBrandsKey             = "GetBrands"
	EndpointGetCategoriesKey         = "GetCategories"
	EndpointGetCategoryAttributesKey = "GetCategoryAttributes"

	// Product endpoints with sellerId
	EndpointGetProductsKey           = "GetProducts"
	EndpointCreateProductsKey        = "CreateProducts"
	EndpointUpdateProductsKey        = "UpdateProducts"
	EndpointDeleteProductsKey        = "DeleteProducts"
	EndpointGetBatchRequestResultKey = "GetBatchRequestResult"
)

// API Endpoints - Inventory Module
const (
	EndpointUpdatePriceInventoryKey = "UpdatePriceInventory"
)

// API Endpoints - Order Module
const (
	EndpointGetOrdersKey            = "GetOrders"
	EndpointUpdatePackageStatusKey  = "UpdatePackageStatus"
	EndpointUpdateTrackingNumberKey = "UpdateTrackingNumber"
	EndpointCancelPackageItemsKey   = "CancelPackageItems"
	EndpointSplitPackageKey         = "SplitPackage"
	EndpointMultiSplitPackageKey    = "MultiSplitPackage"
	EndpointQuantitySplitPackageKey = "QuantitySplitPackage"
	EndpointUpdateBoxInfoKey        = "UpdateBoxInfo"
	EndpointAlternativeDeliveryKey  = "AlternativeDelivery"
	EndpointManualDeliverKey        = "ManualDeliver"
	EndpointManualReturnKey         = "ManualReturn"
	EndpointUpdateCargoProviderKey  = "UpdateCargoProvider"
	EndpointUpdateWarehouseKey      = "UpdateWarehouse"
	EndpointExtendDeliveryDateKey   = "ExtendDeliveryDate"
	EndpointUpdateLaborCostsKey     = "UpdateLaborCosts"
	EndpointDeliveredByServiceKey   = "DeliveredByService"
)

// API Endpoints - Claims Module
const (
	EndpointGetClaimsKey            = "GetClaims"
	EndpointApproveClaimKey         = "ApproveClaim"
	EndpointRejectClaimKey          = "RejectClaim"
	EndpointGetClaimIssueReasonsKey = "GetClaimIssueReasons"
	EndpointGetClaimAuditKey        = "GetClaimAudit"
)

// API Endpoints - Address Module
const (
	EndpointSellerAddressesKey = "SellerAddresses"
)

// API Endpoints - Invoice Module
const (
	EndpointSendInvoiceLinkKey   = "SendInvoiceLink"
	EndpointDeleteInvoiceLinkKey = "DeleteInvoiceLink"
)

// API Endpoints - Common Label Module
const (
	EndpointCreateCommonLabelKey = "CreateCommonLabel"
	EndpointGetCommonLabelKey    = "GetCommonLabel"
)

// API Endpoints - Finance Module
const (
	EndpointGetSettlementsKey         = "GetSettlements"
	EndpointGetCargoInvoiceDetailsKey = "GetCargoInvoiceDetails"
)

// API Endpoints - Member Module
const (
	EndpointGetCountriesKey      = "GetCountries"
	EndpointGetCountryCitiesKey  = "GetCountryCities"
	EndpointGetDomesticCitiesKey = "GetDomesticCities"
)

// API Endpoints - Test Module
const (
	EndpointCreateTestOrderKey          = "CreateTestOrder"
	EndpointUpdateTestOrderStatusKey    = "UpdateTestOrderStatus"
	EndpointTestClaimWaitingInActionKey = "TestClaimWaitingInAction"
)

// API Endpoints - Shipment Module
const (
	EndpointGetShipmentProvidersKey = "GetShipmentProviders"
)

// defaultEndpoints haritası override edilebilir.
var defaultEndpoints = map[string]string{
	// Product Module
	EndpointGetProductsKey:           "/integration/product/sellers/%s/products",
	EndpointCreateProductsKey:        "/integration/product/sellers/%s/products",
	EndpointUpdateProductsKey:        "/integration/product/sellers/%s/products",
	EndpointDeleteProductsKey:        "/integration/product/sellers/%s/products",
	EndpointGetBatchRequestResultKey: "/integration/product/sellers/%s/products/batch-requests/%s",
	EndpointGetBrandsKey:             "/integration/product/brands",
	EndpointGetCategoriesKey:         "/integration/product/product-categories",
	EndpointGetCategoryAttributesKey: "/integration/product/product-categories/%d/attributes",

	// Inventory Module
	EndpointUpdatePriceInventoryKey: "/integration/inventory/sellers/%s/products/price-and-inventory",

	// Order Module
	EndpointGetOrdersKey:            "/integration/order/sellers/%s/orders",
	EndpointUpdatePackageStatusKey:  "/integration/order/sellers/%s/shipment-packages/%d",
	EndpointUpdateTrackingNumberKey: "/integration/order/sellers/%s/shipment-packages/%d/update-tracking-number",
	EndpointCancelPackageItemsKey:   "/integration/order/sellers/%s/shipment-packages/%d/items/unsupplied",
	EndpointSplitPackageKey:         "/integration/order/sellers/%s/shipment-packages/%d/split",
	EndpointMultiSplitPackageKey:    "/integration/order/sellers/%s/shipment-packages/%d/multi-split",
	EndpointQuantitySplitPackageKey: "/integration/order/sellers/%s/shipment-packages/%d/quantity-split",
	EndpointUpdateBoxInfoKey:        "/integration/order/sellers/%s/shipment-packages/%d/box-info",
	EndpointAlternativeDeliveryKey:  "/integration/order/sellers/%s/shipment-packages/%d/alternative-delivery",
	EndpointManualDeliverKey:        "/integration/order/sellers/%s/manual-deliver/%s",
	EndpointManualReturnKey:         "/integration/order/sellers/%s/manual-return/%s",
	EndpointUpdateCargoProviderKey:  "/integration/order/sellers/%s/shipment-packages/%d/cargo-providers",
	EndpointUpdateWarehouseKey:      "/integration/order/sellers/%s/shipment-packages/%d/warehouse",
	EndpointExtendDeliveryDateKey:   "/integration/order/sellers/%s/shipment-packages/%d/extended-agreed-delivery-date",
	EndpointUpdateLaborCostsKey:     "/integration/order/sellers/%s/shipment-packages/%d/labor-costs",
	EndpointDeliveredByServiceKey:   "/integration/order/sellers/%s/shipment-packages/%d/delivered-by-service",

	// Claims Module
	EndpointGetClaimsKey:            "/integration/order/sellers/%s/claims",
	EndpointApproveClaimKey:         "/integration/order/sellers/%s/claims/%s/items/approve",
	EndpointRejectClaimKey:          "/integration/order/sellers/%s/claims/%s/issue",
	EndpointGetClaimIssueReasonsKey: "/integration/order/claim-issue-reasons",
	EndpointGetClaimAuditKey:        "/integration/order/sellers/%s/claims/items/%s/audit",

	// Address Module
	EndpointSellerAddressesKey: "/integration/sellers/%s/addresses",

	// Invoice Module
	EndpointSendInvoiceLinkKey:   "/integration/sellers/%s/seller-invoice-links",
	EndpointDeleteInvoiceLinkKey: "/integration/sellers/%s/seller-invoice-links/delete",

	// Common Label Module
	EndpointCreateCommonLabelKey: "/integration/sellers/%s/common-label/%s",
	EndpointGetCommonLabelKey:    "/integration/sellers/%s/common-label/%s",

	// Finance Module
	EndpointGetSettlementsKey:         "/integration/finance/sellers/%s/settlements",
	EndpointGetCargoInvoiceDetailsKey: "/integration/finance/sellers/%s/cargo-invoice-details/%s",

	// Member Module
	EndpointGetCountriesKey:      "/integration/member/countries",
	EndpointGetCountryCitiesKey:  "/integration/member/countries/%s/cities",
	EndpointGetDomesticCitiesKey: "/integration/member/countries/domestic/%s/cities",

	// Test Module
	EndpointCreateTestOrderKey:          "/integration/test/order/orders/core",
	EndpointUpdateTestOrderStatusKey:    "/integration/test/order/sellers/%s/shipment-packages/%d/status",
	EndpointTestClaimWaitingInActionKey: "/integration/test/order/sellers/%s/claims/waiting-in-action",

	// Shipment Module
	EndpointGetShipmentProvidersKey: "/shipment-providers",
}
