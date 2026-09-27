package trendyol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// UnmarshalJSON accepts both the current package fields and older webhook
// payloads. Presence, rather than a nonzero value, determines precedence.
func (o *Order) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for current, legacy := range map[string]string{
		"shipmentPackageId":    "id",
		"packageGrossAmount":   "grossAmount",
		"packageTotalDiscount": "totalDiscount",
		"packageTyDiscount":    "totalTyDiscount",
		"packageTotalPrice":    "totalPrice",
	} {
		if _, exists := fields[current]; !exists {
			if value, ok := fields[legacy]; ok {
				fields[current] = value
			}
		}
	}
	if err := normalizeIntegerField(fields, "cargoTrackingNumber"); err != nil {
		return err
	}
	// ETGB dates are numeric millisecond timestamps in current responses.
	// Keep the public string field for existing callers.
	if value := bytes.TrimSpace(fields["etgbDate"]); len(value) > 0 && value[0] != '"' && !bytes.Equal(value, []byte("null")) {
		var number json.Number
		if err := json.Unmarshal(value, &number); err != nil {
			return fmt.Errorf("etgbDate: %w", err)
		}
		fields["etgbDate"], _ = json.Marshal(number.String())
	}
	type plainOrder Order
	return decodeOrderFields(fields, (*plainOrder)(o))
}

// UnmarshalJSON preserves the numeric Phone API while accepting the string,
// empty, null and masked phone representations returned by Trendyol.
func (a *OrderAddress) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	// A redacted phone is unavailable. Never turn its visible digits into a
	// partial phone number; use the same zero value as an empty/null phone.
	var phone string
	if json.Unmarshal(fields["phone"], &phone) == nil && strings.Contains(phone, "*") {
		fields["phone"] = json.RawMessage("0")
	}
	if err := normalizeIntegerField(fields, "phone"); err != nil {
		return err
	}
	type plainAddress OrderAddress
	return decodeOrderFields(fields, (*plainAddress)(a))
}

// UnmarshalJSON maps current wire names to the existing public Go fields.
// The new fields take precedence when a transitional response includes both.
func (l *OrderLine) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for current, legacy := range map[string]string{
		"lineId":             "id",
		"stockCode":          "merchantSku",
		"sellerId":           "merchantId",
		"lineGrossAmount":    "amount",
		"lineSellerDiscount": "discount",
		"lineTyDiscount":     "tyDiscount",
		"vatRate":            "vatBaseAmount",
		"lineUnitPrice":      "price",
	} {
		if value, exists := fields[current]; exists {
			fields[legacy] = value
		}
	}
	type plainLine OrderLine
	return decodeOrderFields(fields, (*plainLine)(l))
}

func (d *DiscountDetail) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if value, exists := fields["lineItemSellerDiscount"]; exists {
		fields["lineItemDiscount"] = value
	}
	type plainDiscount DiscountDetail
	return decodeOrderFields(fields, (*plainDiscount)(d))
}

// UnmarshalJSON keeps ListLegacy usable with the current order endpoint while
// retaining support for the original shipment-package JSON representation.
func (p *ShipmentPackage) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	_, hasID := fields["shipmentPackageId"]
	_, hasAddress := fields["shipmentAddress"]
	if !hasID && !hasAddress {
		type plainPackage ShipmentPackage
		return json.Unmarshal(data, (*plainPackage)(p))
	}
	var order Order
	if err := json.Unmarshal(data, &order); err != nil {
		return err
	}
	*p = ShipmentPackage{
		ID: order.ID, SupplierID: int(order.SupplierID), Status: order.Status,
		CreationDate: order.OrderDate, LastModifiedDate: order.LastModifiedDate,
		BuyerID: order.CustomerID, CargoProviderName: order.CargoProviderName,
		ShippingAddress: legacyOrderAddress(order.ShipmentAddress),
		BillingAddress:  legacyOrderAddress(order.InvoiceAddress),
	}
	if p.Status == "" {
		p.Status = order.ShipmentPackageStatus
	}
	if order.CargoTrackingNumber != 0 {
		p.CargoTrackingNumber = strconv.FormatInt(order.CargoTrackingNumber, 10)
	}
	for _, line := range order.Lines {
		p.Lines = append(p.Lines, ShipmentLine{
			LineID: line.ID, Barcode: line.Barcode, Quantity: line.Quantity,
			Price: line.Price, ProductName: line.ProductName,
			MerchantSKU: line.MerchantSKU, PackageID: order.ID,
		})
	}
	return nil
}

func legacyOrderAddress(address *OrderAddress) *Address {
	if address == nil {
		return nil
	}
	return &Address{
		ID: int(address.ID), Country: address.CountryCode,
		City: address.City, CityCode: address.CityCode,
		District: address.District, DistrictID: address.DistrictID,
		PostCode: address.PostalCode, Address: address.Address1,
		FullAddress: address.FullAddress,
	}
}

func normalizeIntegerField(fields map[string]json.RawMessage, name string) error {
	value := bytes.TrimSpace(fields[name])
	if len(value) == 0 || value[0] != '"' {
		return nil
	}
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return err
	}
	if text == "" {
		fields[name] = json.RawMessage("0")
		return nil
	}
	number, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return fmt.Errorf("%s: expected an integer or numeric string", name)
	}
	fields[name] = json.RawMessage(strconv.FormatInt(number, 10))
	return nil
}

func decodeOrderFields(fields map[string]json.RawMessage, result interface{}) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}
