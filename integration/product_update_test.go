//go:build integration
// +build integration

package trendyol_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/vahaponur/trendyol-go"
)

// TestProductUpdateInteractive interaktif olarak ürün güncelleme senaryosu çalıştırır.
// 1) Kullanıcıdan barkod ister
// 2) Mevcut ürünü API'den çeker ve JSON olarak yazdırır
// 3) Kullanıcıdan yeni JSON'ı stdin üzerinden alır
// 4) Eski ve yeni değerleri karşılaştırarak rapor üretir
// 5) Kullanıcı onay verirse Update API çağrısını yapar ve batch tamamlanmasını bekler
// Not: Bu fonksiyon production ortamında doğrudan güncelleme yapabileceği için
// kullanıcı onayı almadan hiçbir değişikliği push etmez.
func TestProductUpdateInteractive(t *testing.T) {
	fmt.Println("\n=== ÜRÜN GÜNCELLEME TESTİ (PRODUCTİON) ===")
	fmt.Println("DİKKAT: Bu test GERÇEK ürünleri günceller!")
	fmt.Println("=========================================")

	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	reader := bufio.NewReader(os.Stdin)

	// 1) Barkod iste
	fmt.Print("Güncellemek istediğiniz ürünün barkodunu girin: ")
	barcode, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Barkod okuma hatası: %v", err)
	}
	barcode = strings.TrimSpace(barcode)
	if barcode == "" {
		t.Fatal("Barkod boş olamaz!")
	}

	fmt.Printf("\nBarkod: %s ile ürün aranıyor...\n", barcode)

	// 2) Mevcut ürünü çek
	current, err := client.Products.GetByBarcode(ctx, barcode)
	if err != nil {
		t.Fatalf("HATA: Ürün getirilemedi: %v\nBarkod doğru mu? API erişiminiz var mı?", err)
	}

	fmt.Println("\n✅ Ürün bulundu!")

	// JSON olarak yazdır
	fmt.Println("\n--- MEVCUT ÜRÜN (API'DEN GELEN) ---")
	currentJSON, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		t.Fatalf("JSON oluşturma hatası: %v", err)
	}
	fmt.Println(string(currentJSON))
	fmt.Println("--- MEVCUT ÜRÜN SONU ---")

	// 3) Kullanıcıdan yeni JSON al
	fmt.Println("\n📝 GÜNCELLEME JSON'I:")
	fmt.Println("Aşağıya güncellenmiş ürün JSON'ını TEK SATIRDA yapıştırın ve Enter'a basın:")
	fmt.Println("(İpucu: JSON'ı bir editörde hazırlayıp tek satıra çevirin)")
	fmt.Print("> ")

	jsonLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("JSON okuma hatası: %v", err)
	}
	jsonLine = strings.TrimSpace(jsonLine)
	if jsonLine == "" {
		t.Fatal("JSON boş olamaz! Test iptal edildi.")
	}

	// JSON'ı parse et
	var newProd Product
	if err := json.Unmarshal([]byte(jsonLine), &newProd); err != nil {
		t.Fatalf("HATA: JSON parse edilemedi: %v\nGirdiğiniz JSON geçerli mi?", err)
	}

	// Barkod kontrolü - güvenlik için
	if newProd.Barcode != barcode {
		t.Fatalf("HATA: JSON'daki barkod (%s) ile aradığınız barkod (%s) uyuşmuyor!", newProd.Barcode, barcode)
	}

	// 4) Karşılaştırma & rapor
	fmt.Println("\n🔍 Değişiklikler analiz ediliyor...")
	changed, same, missing := diffProductMaps(current, jsonLine)

	fmt.Println("\n=== KARŞILAŞTIRMA RAPORU ===")

	if len(changed) == 0 {
		fmt.Println("✅ Değişen alan yok")
	} else {
		fmt.Printf("\n📝 DEĞİŞEN ALANLAR (%d adet):\n", len(changed))
		for _, c := range changed {
			oldVal := getFieldValue(current, c)
			newVal := getFieldValueFromJSON(jsonLine, c)
			fmt.Printf("  • %s: %v → %v\n", c, oldVal, newVal)
		}
	}

	if len(same) > 0 {
		fmt.Printf("\n✅ AYNI KALAN ALANLAR (%d adet):\n", len(same))
		for _, s := range same {
			fmt.Printf("  • %s\n", s)
		}
	}

	if len(missing) > 0 {
		fmt.Printf("\n❌ EKSİK ZORUNLU ALANLAR (%d adet):\n", len(missing))
		for _, m := range missing {
			fmt.Printf("  • %s (ZORUNLU!)\n", m)
		}
	}

	fmt.Println("\n=== RAPOR SONU ===")

	// Zorunlu alan kontrolü
	if len(missing) > 0 {
		t.Fatal("\n❌ HATA: Zorunlu alanlar eksik! Güncelleme GÖNDERİLMEDİ.")
	}

	if len(changed) == 0 {
		fmt.Println("\n✅ Hiçbir değişiklik yok, güncelleme gerekmez.")
		t.Skip("Değişiklik olmadığı için test atlandı")
	}

	// 5) Kullanıcıdan onay al
	fmt.Printf("\n⚠️  DİKKAT: %d alan değişecek!\n", len(changed))
	fmt.Print("Güncellemeyi Trendyol'a göndermek istiyor musunuz? (evet/hayır): ")

	answer, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Onay okuma hatası: %v", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "evet" && answer != "e" && answer != "yes" && answer != "y" {
		fmt.Println("\n❌ Kullanıcı iptal etti, güncelleme yapılmadı.")
		t.Skip("Kullanıcı tarafından iptal edildi")
	}

	// Update çağrısı
	fmt.Println("\n📤 Güncelleme gönderiliyor...")
	updateResp, err := client.Products.Update(ctx, []Product{newProd})
	if err != nil {
		t.Fatalf("❌ HATA: Update çağrısı başarısız: %v", err)
	}

	fmt.Printf("✅ Batch oluşturuldu: %s\n", updateResp.BatchRequestID)
	fmt.Println("⏳ Batch işleniyor, lütfen bekleyin...")

	// Batch takibi
	if err := waitBatchSuccess(ctx, client, updateResp.BatchRequestID); err != nil {
		t.Fatalf("❌ HATA: Batch tamamlanamadı: %v", err)
	}

	fmt.Println("\n🎉 ÜRÜN BAŞARIYLA GÜNCELLENDİ!")
}

// diffProductMaps eski ürün struct'ı ile yeni JSON arasındaki farkları döner.
func diffProductMaps(oldProd *Product, newJSON string) (changed, same, missing []string) {
	var oldMap, newMap map[string]interface{}
	oldBytes, _ := json.Marshal(oldProd)
	_ = json.Unmarshal(oldBytes, &oldMap)
	_ = json.Unmarshal([]byte(newJSON), &newMap)

	// Zorunlu alan listesi – Trendyol dökümantasyonundan
	required := []string{
		"barcode",
		"title",
		"productMainId",
		"brandId",
		"categoryId",
		"stockCode",
		"dimensionalWeight",
		"description",
		"currencyType",
		"cargoCompanyId",
		"vatRate",
		"images",
		"attributes",
	}

	// Tüm alanları topla
	fieldSet := map[string]struct{}{}
	for k := range oldMap {
		fieldSet[k] = struct{}{}
	}
	for k := range newMap {
		fieldSet[k] = struct{}{}
	}

	// Karşılaştır
	for field := range fieldSet {
		oldVal, okOld := oldMap[field]
		newVal, okNew := newMap[field]

		switch {
		case okOld && okNew:
			if reflect.DeepEqual(oldVal, newVal) {
				same = append(same, field)
			} else {
				changed = append(changed, field)
			}
		case okOld && !okNew:
			// Alan eski üründe var ama yenide yok → sadece zorunluysa problem
		case !okOld && okNew:
			// Yeni alan ekleniyor
			changed = append(changed, field)
		}
	}

	// Zorunlu alan kontrolü
	for _, req := range required {
		if _, ok := newMap[req]; !ok {
			missing = append(missing, req)
		}
	}

	return
}

// getFieldValue struct'tan alan değerini alır (basit gösterim için)
func getFieldValue(p *Product, field string) interface{} {
	m := map[string]interface{}{}
	data, _ := json.Marshal(p)
	_ = json.Unmarshal(data, &m)
	if val, ok := m[field]; ok {
		return val
	}
	return nil
}

// getFieldValueFromJSON JSON string'den alan değerini alır
func getFieldValueFromJSON(jsonStr, field string) interface{} {
	m := map[string]interface{}{}
	_ = json.Unmarshal([]byte(jsonStr), &m)
	if val, ok := m[field]; ok {
		return val
	}
	return nil
}
