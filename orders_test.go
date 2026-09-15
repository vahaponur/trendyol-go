package trendyol

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Uses only current API field names, so transitional responses containing both
// old and new names cannot conceal missing mappings. All values are synthetic.
const currentOrderJSON = `{
 "shipmentPackageId":3330111111,"supplierId":123,"orderNumber":"TEST-1",
 "packageGrossAmount":200,"packageTotalDiscount":30,"packageTyDiscount":10,
 "packageSellerDiscount":20,"packageTotalPrice":170,"paymentMethod":"Kredi Kartı",
 "customerId":456,"orderDate":1789000000000,"status":"Created",
 "shipmentPackageStatus":"Created","cargoTrackingNumber":7280027504111111,
 "shipmentAddress":{"id":12,"phone":"333333333","city":"İstanbul","address1":"Test","countryCode":"TR","postalCode":"34000"},
 "invoiceAddress":{"phone":null,"taxOffice":"Test","taxNumber":"1234567890","eInvoiceAvailable":true},
 "etgbDate":1789000000000,"3pByTrendyol":true,
 "lines":[{"lineId":4765111111,"quantity":2,"stockCode":"TEST-STOCK",
 "contentId":1239111111,"sellerId":123,"lineGrossAmount":100,
 "lineTotalDiscount":15,"lineSellerDiscount":10,"lineTyDiscount":5,
 "vatRate":20,"lineUnitPrice":85,"barcode":"TEST-BARCODE",
 "discountDetails":[{"lineItemPrice":85,"lineItemSellerDiscount":10,"lineItemTyDiscount":5}]}]
}`

func assertCurrentOrder(t *testing.T, o Order) {
	t.Helper()
	if o.ID != 3330111111 || o.TotalPrice != 170 || o.GrossAmount != 200 || o.TotalDiscount != 30 || o.TotalTyDiscount != 10 || o.SellerDiscount != 20 {
		t.Fatalf("package identity or totals lost: %+v", o)
	}
	if o.ShipmentAddress.Phone != 333333333 || o.InvoiceAddress.Phone != 0 || o.EtgbDate != "1789000000000" || !o.ThreePbyTrendyol {
		t.Fatalf("scalar fields decoded incorrectly: %+v", o)
	}
	if o.PaymentMethod != "Kredi Kartı" || o.InvoiceAddress.TaxNumber != "1234567890" || !o.InvoiceAddress.EInvoiceAvailable {
		t.Fatal("invoice fields lost")
	}
	if len(o.Lines) != 1 {
		t.Fatalf("lines=%d", len(o.Lines))
	}
	l := o.Lines[0]
	if l.ID != 4765111111 || l.MerchantSKU != "TEST-STOCK" || l.MerchantID != 123 || l.ContentID != 1239111111 || l.ProductCode != 0 {
		t.Fatalf("line identity lost or content ID confused with variant ID: %+v", l)
	}
	if l.Amount != 100 || l.TotalDiscount != 15 || l.Discount != 10 || l.TyDiscount != 5 || l.Price != 85 || l.VATBaseAmount != 20 || l.DiscountDetails[0].LineItemDiscount != 10 {
		t.Fatalf("line prices lost: %+v", l)
	}
}

func TestOrderCurrentResponse(t *testing.T) {
	var o Order
	if err := json.Unmarshal([]byte(currentOrderJSON), &o); err != nil {
		t.Fatal(err)
	}
	assertCurrentOrder(t, o)
	encoded, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip Order
	if err = json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(o, roundTrip) {
		t.Fatal("public JSON no longer round trips")
	}
}

func TestOrderLegacyWebhookAndCurrentPrecedence(t *testing.T) {
	var o Order
	payload := `{"id":11,"grossAmount":100,"totalDiscount":10,"totalTyDiscount":2,"totalPrice":90,"shipmentAddress":{"phone":333333333},"etgbDate":"1789000000000","lines":[{"id":22,"merchantSku":"OLD","productCode":33,"merchantId":44,"amount":100,"discount":10,"tyDiscount":2,"price":88,"vatBaseAmount":20,"discountDetails":[{"lineItemDiscount":10}]}]}`
	if err := json.Unmarshal([]byte(payload), &o); err != nil {
		t.Fatal(err)
	}
	if o.ID != 11 || o.TotalPrice != 90 || o.GrossAmount != 100 || o.TotalDiscount != 10 || o.TotalTyDiscount != 2 || o.Lines[0].ID != 22 || o.Lines[0].ProductCode != 33 || o.Lines[0].Price != 88 || o.Lines[0].DiscountDetails[0].LineItemDiscount != 10 {
		t.Fatalf("legacy fields lost: %+v", o)
	}
	o = Order{}
	payload = `{"id":11,"shipmentPackageId":0,"grossAmount":100,"packageGrossAmount":0,"totalPrice":90,"packageTotalPrice":0,"totalDiscount":10,"packageTotalDiscount":0,"totalTyDiscount":2,"packageTyDiscount":0,"lines":[{"id":22,"lineId":0,"merchantSku":"OLD","stockCode":"","amount":100,"lineGrossAmount":0,"discount":10,"lineSellerDiscount":0,"tyDiscount":2,"lineTyDiscount":0,"price":88,"lineUnitPrice":0,"vatBaseAmount":20,"vatRate":0,"discountDetails":[{"lineItemDiscount":10,"lineItemSellerDiscount":0}]}]}`
	if err := json.Unmarshal([]byte(payload), &o); err != nil {
		t.Fatal(err)
	}
	if o.ID != 0 || o.TotalPrice != 0 || o.GrossAmount != 0 || o.TotalDiscount != 0 || o.TotalTyDiscount != 0 || o.Lines[0].ID != 0 || o.Lines[0].MerchantSKU != "" || o.Lines[0].Amount != 0 || o.Lines[0].Discount != 0 || o.Lines[0].TyDiscount != 0 || o.Lines[0].Price != 0 || o.Lines[0].VATBaseAmount != 0 || o.Lines[0].DiscountDetails[0].LineItemDiscount != 0 {
		t.Fatalf("current zero values overridden by stale fields: %+v", o)
	}
}

func TestOrderScalarVariants(t *testing.T) {
	for _, value := range []string{`null`, `""`, `0`, `"0"`, `333333333`, `"333333333"`} {
		t.Run(value, func(t *testing.T) {
			var o Order
			if err := json.Unmarshal([]byte(`{"shipmentAddress":{"phone":`+value+`},"cargoTrackingNumber":`+value+`}`), &o); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(value, "333333333") && (o.ShipmentAddress.Phone != 333333333 || o.CargoTrackingNumber != 333333333) {
				t.Fatal("numeric string lost")
			}
		})
	}
	for _, value := range []string{`true`, `{}`, `"invalid"`, `"9223372036854775808"`} {
		var o Order
		if err := json.Unmarshal([]byte(`{"shipmentAddress":{"phone":`+value+`}}`), &o); err == nil {
			t.Fatalf("accepted invalid phone %s", value)
		}
	}
}

type orderTransport func(*http.Request) (*http.Response, error)

func (f orderTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func testOrderClient(t *testing.T, sandbox bool, f orderTransport, opts ...ClientOption) *Client {
	t.Helper()
	opts = append(opts, WithHTTPClient(&http.Client{Transport: f}), WithRetryConfig(0, 0))
	return NewClient("123", "test-key", "test-secret", sandbox, opts...)
}
func orderResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestOrdersListV2RequestAndLegacyAdapter(t *testing.T) {
	start := time.UnixMilli(1789000000000)
	end := start.Add(time.Hour)
	opts := ListOrdersOptions{Page: 1, Size: 50, Status: "Created", StartDate: &start, EndDate: &end, OrderByField: "PackageLastModifiedDate", OrderByDirection: "DESC", OrderNumber: "TEST-1", ShipmentPackageIDs: []int64{3330111111, 3330111112}}
	wantQuery := url.Values{"page": {"1"}, "size": {"50"}, "status": {"Created"}, "startDate": {"1789000000000"}, "endDate": {"1789003600000"}, "orderByField": {"PackageLastModifiedDate"}, "orderByDirection": {"DESC"}, "orderNumber": {"TEST-1"}, "shipmentPackageIds": {"3330111111", "3330111112"}}
	for _, sandbox := range []bool{false, true} {
		client := testOrderClient(t, sandbox, func(r *http.Request) (*http.Response, error) {
			wantHost := "apigw.trendyol.com"
			if sandbox {
				wantHost = "stageapigw.trendyol.com"
			}
			if r.Method != http.MethodGet || r.URL.Host != wantHost || r.URL.Path != "/integration/order/sellers/123/v2/orders" {
				t.Fatalf("wrong request: %s %s", r.Method, r.URL)
			}
			if !reflect.DeepEqual(r.URL.Query(), wantQuery) {
				t.Fatalf("query=%v", r.URL.Query())
			}
			user, pass, ok := r.BasicAuth()
			if !ok || user != "test-key" || pass != "test-secret" || r.Header.Get("User-Agent") != "123 - SelfIntegration" {
				t.Fatal("authentication headers changed")
			}
			return orderResponse(`{"page":1,"size":50,"totalElements":51,"totalPages":2,"content":[` + currentOrderJSON + `]}`), nil
		})
		orders, page, err := client.Orders.List(context.Background(), opts)
		if err != nil {
			t.Fatal(err)
		}
		if len(orders) != 1 || page.Page != 1 || page.TotalElement != 51 || page.TotalPages != 2 {
			t.Fatal("pagination lost")
		}
		assertCurrentOrder(t, orders[0])
		packages, legacyPage, err := client.Orders.ListLegacy(context.Background(), opts)
		if err != nil {
			t.Fatal(err)
		}
		p := packages[0]
		if !reflect.DeepEqual(page, legacyPage) || p.ID != orders[0].ID || p.BuyerID != 456 || p.CargoTrackingNumber != "7280027504111111" || p.ShippingAddress.City != "İstanbul" || p.Lines[0].LineID != 4765111111 || p.Lines[0].Price != 85 || p.Lines[0].MerchantSKU != "TEST-STOCK" {
			t.Fatalf("legacy adapter lost data: %+v", p)
		}
	}
}

func TestOrdersStreamCursorAndEndpointOverrides(t *testing.T) {
	start := time.UnixMilli(1789000000000)
	end := start.Add(time.Hour)
	cursor := "opaque+/=?& cursor"
	calls := 0
	client := testOrderClient(t, false, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/custom/123/stream" {
			t.Fatalf("wrong stream path: %s", r.URL.Path)
		}
		want := url.Values{"size": {"50"}, "packageItemStatuses": {"Created,Picking"}, "lastModifiedStartDate": {"1789000000000"}, "lastModifiedEndDate": {"1789003600000"}}
		if calls == 2 {
			want.Set("nextCursor", cursor)
		}
		if !reflect.DeepEqual(r.URL.Query(), want) {
			t.Fatalf("stream query=%v want=%v", r.URL.Query(), want)
		}
		if calls == 1 {
			return orderResponse(`{"hasMore":true,"nextCursor":"opaque+/=?& cursor","size":50,"content":[` + currentOrderJSON + `]}`), nil
		}
		return orderResponse(`{"hasMore":false,"nextCursor":null,"size":50,"content":[]}`), nil
	}, WithEndpointOverrides(map[string]string{EndpointGetOrdersStreamKey: "/custom/%s/stream"}))
	opts := ListOrdersStreamOptions{Size: 50, PackageItemStatuses: "Created,Picking", LastModifiedStartDate: &start, LastModifiedEndDate: &end}
	orders, page, err := client.Orders.ListStream(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	assertCurrentOrder(t, orders[0])
	if !page.HasMore || page.NextCursor != cursor {
		t.Fatal("cursor lost")
	}
	opts.NextCursor = page.NextCursor
	orders, page, err = client.Orders.ListStream(context.Background(), opts)
	if err != nil || page.HasMore || len(orders) != 0 || calls != 2 {
		t.Fatalf("stream completion failed: %v", err)
	}
}

func TestOrdersStreamDefaultAndListOverride(t *testing.T) {
	client := testOrderClient(t, true, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "stageapigw.trendyol.com" {
			t.Fatal("sandbox host changed")
		}
		switch r.URL.Path {
		case "/integration/order/sellers/123/orders/stream":
			if r.URL.RawQuery != "" {
				t.Fatalf("zero options should use server defaults: %s", r.URL.RawQuery)
			}
			return orderResponse(`{"content":[],"hasMore":false,"size":50}`), nil
		case "/custom/123/orders":
			return orderResponse(`{"content":[]}`), nil
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
			return nil, nil
		}
	}, WithEndpointOverrides(map[string]string{EndpointGetOrdersKey: "/custom/%s/orders"}))
	if _, _, err := client.Orders.ListStream(context.Background(), ListOrdersStreamOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.Orders.List(context.Background(), ListOrdersOptions{Size: 50}); err != nil {
		t.Fatal(err)
	}
}

func TestOrdersErrorsAndCargoEndpoint(t *testing.T) {
	client := testOrderClient(t, false, func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPut {
			if r.URL.Path != "/integration/order/sellers/123/shipment-packages/3330111111/cargo-providers" {
				t.Fatal("cargo endpoint changed")
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["cargoProvider"] != "ARASMP" {
				t.Fatal("cargo payload changed")
			}
			return orderResponse(""), nil
		}
		response := orderResponse(`{"message":"invalid filter"}`)
		response.StatusCode = 400
		return response, nil
	})
	for _, stream := range []bool{false, true} {
		var err error
		if stream {
			_, _, err = client.Orders.ListStream(context.Background(), ListOrdersStreamOptions{})
		} else {
			_, _, err = client.Orders.List(context.Background(), ListOrdersOptions{})
		}
		var apiError *Error
		if !errors.As(err, &apiError) || apiError.StatusCode != 400 {
			t.Fatalf("lost API error: %v", err)
		}
	}
	if err := client.Orders.UpdateCargoProvider(context.Background(), 3330111111, "ARASMP"); err != nil {
		t.Fatal(err)
	}
}
