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
		Status: "Created",
		Page:   0,
		Size:   50,
	}

	orders, page, err := client.Orders.List(ctx, opts)
	if err != nil {
		t.Fatalf("Sipariş listesi alınamadı: %v", err)
	}

	fmt.Printf("--- Toplam paket: %d | Sayfa %d/%d ---\n", page.TotalElement, page.Page+1, page.TotalPages)

	if len(orders) > 0 {
		b, _ := json.MarshalIndent(orders[0], "", "  ")
		fmt.Printf("--- İlk Paket Örneği ---\n%s\n", string(b))
	}
}
