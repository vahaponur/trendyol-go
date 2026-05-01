//go:build integration
// +build integration

package trendyol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	. "github.com/vahaponur/trendyol-go"
)

// TestOrdersListBasic getShipmentPackages servisinin SDK implementasyonunu temel parametrelerle çağırır.
func TestOrdersListBasic(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := ListOrdersOptions{
		Page: 0,
		Size: 50,
	}

	orders, page, err := client.Orders.List(ctx, opts)
	if err != nil {
		t.Fatalf("Sipariş listesi alınamadı: %v", err)
	}

	fmt.Printf("--- Toplam paket: %d | Sayfa %d/%d ---\n", len(orders), page.Page+1, page.TotalPages)

	if len(orders) > 0 {
		b, _ := json.MarshalIndent(orders[0], "", "  ")
		fmt.Printf("--- İlk Paket Örneği ---\n%s\n", string(b))
	}
}

// TestOrdersListWithDateRange tarih aralığı ile sipariş listesini test eder
func TestOrdersListWithDateRange(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 7 Temmuz 2025 - 21 Temmuz 2025
	startDate := time.Date(2025, 7, 7, 0, 0, 0, 0, time.FixedZone("GMT+3", 3*60*60))
	endDate := time.Date(2025, 7, 21, 23, 59, 59, 999999999, time.FixedZone("GMT+3", 3*60*60))

	opts := ListOrdersOptions{
		Status:    "Created",
		StartDate: &startDate,
		EndDate:   &endDate,
		Page:      0,
		Size:      50,
	}

	fmt.Printf("--- Tarih Aralığı Testi ---\n")
	fmt.Printf("Başlangıç: %s (Unix Milli: %d)\n", startDate.Format(time.RFC3339), startDate.UnixMilli())
	fmt.Printf("Bitiş: %s (Unix Milli: %d)\n", endDate.Format(time.RFC3339), endDate.UnixMilli())

	orders, page, err := client.Orders.List(ctx, opts)
	if err != nil {
		t.Fatalf("Tarih aralıklı sipariş listesi alınamadı: %v", err)
	}

	fmt.Printf("--- Toplam paket: %d | Sayfa %d/%d ---\n", page.TotalElement, page.Page+1, page.TotalPages)

	if len(orders) > 0 {
		fmt.Printf("--- Bulunan %d sipariş ---\n", len(orders))
		for i, order := range orders {
			orderDate := time.UnixMilli(order.OrderDate)
			fmt.Printf("%d. Sipariş No: %s, Tarih: %s\n", i+1, order.OrderNumber, orderDate.Format(time.RFC3339))
		}
	} else {
		fmt.Println("--- Belirtilen tarih aralığında sipariş bulunamadı ---")
	}
}
