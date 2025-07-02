//go:build integration
// +build integration

package trendyol_test

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestGetCategoryAttributes belirli bir kategorinin zorunlu/opsiyonel özelliklerini listeler.
// Çıktı terminalde ID, isim ve zorunlu bilgilerini gösterir.
func TestGetCategoryAttributes(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const catID = 2927 // Örnek: Davetiye kategorisi

	attrs, err := client.Categories.GetCategoryAttributes(ctx, catID)
	if err != nil {
		t.Fatalf("Kategori özellikleri alınamadı: %v", err)
	}

	fmt.Printf("--- Kategori %d Özellik Listesi (%d adet) ---\n", catID, len(attrs))
	for _, a := range attrs {
		fmt.Printf("\nID=%d | İsim=%s | Zorunlu=%v | AllowCustom=%v | DeğerSayısı=%d\n", a.AttributeID, a.AttributeName, a.Required, a.AllowCustomValue, len(a.AttributeValues))
		if len(a.AttributeValues) > 0 {
			fmt.Println("-- Değerler --")
			for _, v := range a.AttributeValues {
				fmt.Printf("   • ValueID=%d | Value=%s\n", v.AttributeValueID, v.Value)
			}
		}
	}
}
