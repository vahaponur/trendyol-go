//go:build integration
// +build integration

package trendyol_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/vahaponur/trendyol-go"

	"github.com/joho/godotenv"
)

// -----------------------------------------------------------------------------
//  Test verilerini kolayca değiştirebilmek için başlangıçta tanımlıyoruz
// -----------------------------------------------------------------------------

var (
	// Test ürünü – değerleri ihtiyacınıza göre güncelleyebilirsiniz
	testProduct = Product{
		Barcode:       "TEST-BARCODE-001",
		Title:         "Go SDK Otomasyon Hoodie",
		ProductMainID: "GO-SDK-TEST-001",
		BrandID:       2209541,
		CategoryID:    2927,
		Quantity:      10,
		StockCode:     "STK-GO-001",
		Description:   "Go SDK ile otomatik test için oluşturulan hoodie ürünü. Kaliteli pamuklu kumaş.",
		ListPrice:     299.90,
		SalePrice:     149.90,
		CurrencyType:  "TRY",
		VATRate:       20,
		Images:        []ProductImage{{URL: "https://images.unsplash.com/photo-1556821840-3a63f95609a7?ixlib=rb-4.0.3&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D&auto=format&fit=crop&w=1000&q=80"}},
		Attributes:    []ProductAttribute{{AttributeID: 1192, AttributeValueID: 10617344}},
	}
)

// Komut satırından barkod geçmek için
var barcodeFlag = flag.String("barcode", "", "Tekil ürün testi için barkod belirt")
var sellerIDFlag = flag.String("sellerid", "", "Başka satıcının ürünlerini test etmek için seller ID belirt")

// Silinecek ürünler için virgülle ayrılmış barkod listesi
var deleteBarcodesFlag = flag.String("delete", "", "Silinecek ürün barkodları (virgülle ayrılmış)")

// loadEnv çağrısı tüm testler başlamadan önce .env dosyasını okur
func init() {
	// godotenv dosyayı bulamazsa hata döndürmez; CI/CD ortamlarında sorun yaratmasın diye
	// integration/ klasöründen test çalıştığımız için parent directory'den .env'i al
	_ = godotenv.Load("../.env")
}

// newTestClient testlerde kullanılacak Trendyol Client'ını ortam değişkenlerinden üretir
func newTestClient(t *testing.T) *Client {
	sellerID := os.Getenv("SELLER_ID")
	apiKey := os.Getenv("API_KEY")
	apiSecret := os.Getenv("API_SECRET")

	if sellerID == "" || apiKey == "" || apiSecret == "" {
		t.Skip("SELLER_ID, API_KEY, API_SECRET env değişkenleri tanımlı değil, entegrasyon testleri atlandı")
	}

	// Testler production ortamında çalışacak, sandbox=false
	return NewClient(sellerID, apiKey, apiSecret, false)
}

// waitBatchSuccess belirli aralıklarla batch durumu "COMPLETED" olana kadar sorgular
func waitBatchSuccess(ctx context.Context, client *Client, batchID string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fmt.Printf("Batch takip başlatıldı: %s\n", batchID)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := client.Products.GetBatchStatus(ctx, batchID)
			if err != nil {
				return err
			}

			// Durumu her döngüde logla
			fmt.Printf("BatchStatus=%s | ItemCount=%d | Failed=%d\n", status.Status, status.ItemCount, status.FailedItemCount)

			if status.Status == "COMPLETED" {
				// Tamamlandı; başarısız kalemler varsa detaylarını yazdır
				if status.FailedItemCount > 0 {
					for _, it := range status.Items {
						if it.Status != "SUCCEEDED" {
							failMsg := strings.Join(it.FailureReasons, "; ")
							itemBytes, _ := json.Marshal(it.RequestItem)
							fmt.Printf("❌ HATA | Item=%s | Reasons=%s\n", string(itemBytes), failMsg)
						}
					}
					return fmt.Errorf("batch tamamlandı ancak %d hata var", status.FailedItemCount)
				}
				fmt.Println("✅ Batch başarıyla tamamlandı")
				return nil
			}
		}
	}
}

// -----------------------------------------------------------------------------
//  Yardımcı fonksiyonlar
// -----------------------------------------------------------------------------

// min iki int değerden küçük olanı döner (Go 1.21 öncesi sürümler için kendi implementasyonumuz)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ensureTestProductUploaded: testProduct mevcut değilse oluşturur ve batch'in tamamlanmasını bekler.
func ensureTestProductUploaded(t *testing.T, client *Client, ctx context.Context) {
	// Ürün zaten varsa çık
	if _, err := client.Products.GetByBarcode(ctx, testProduct.Barcode); err == nil {
		return
	}

	createResp, err := client.Products.Create(ctx, []Product{testProduct})
	if err != nil {
		t.Fatalf("ensureTestProductUploaded: ürün oluşturulamadı: %v", err)
	}
	if createResp.BatchRequestID == "" {
		t.Fatalf("ensureTestProductUploaded: BatchRequestID boş döndü")
	}
	if err := waitBatchSuccess(ctx, client, createResp.BatchRequestID); err != nil {
		t.Fatalf("ensureTestProductUploaded: batch tamamlanamadı: %v", err)
	}
}

// -----------------------------------------------------------------------------
//  Testler
// -----------------------------------------------------------------------------

// TestProductUpload sadece ürün yükleme ve batch kontrolünü doğrular.
func TestProductUpload(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Println("--- Ürün yükleme testi başlatıldı ---")
	// Benzersiz barkod üreterek çakışmaları engelleyelim
	p := testProduct
	now := time.Now().Unix()
	p.Barcode = fmt.Sprintf("%s-%d", p.Barcode, now)
	p.StockCode = fmt.Sprintf("%s-%d", p.StockCode, now)
	p.ProductMainID = fmt.Sprintf("%s-%d", p.ProductMainID, now)

	createResp, err := client.Products.Create(ctx, []Product{p})
	if createResp != nil {
		fmt.Printf("BatchRequestID: %s\n", createResp.BatchRequestID)
	}
	if err != nil {
		t.Fatalf("Ürün yüklenemedi: %v", err)
	}
	if createResp.BatchRequestID == "" {
		t.Fatalf("BatchRequestID boş döndü")
	}
	if err := waitBatchSuccess(ctx, client, createResp.BatchRequestID); err != nil {
		t.Fatalf("Batch tamamlanamadı: %v", err)
	}
}

// TestProductGetSingle barkod ile tekil ürün çekmeyi test eder.
func TestProductGetSingle(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	barcode := testProduct.Barcode
	if *barcodeFlag == "" {
		// testProduct'ın varlığını garanti altına al
		ensureTestProductUploaded(t, client, ctx)
	} else {
		barcode = *barcodeFlag
	}

	product, err := client.Products.GetByBarcode(ctx, barcode)
	if err != nil {
		t.Fatalf("Ürün getirilemedi: %v", err)
	}

	// Ürün detaylarını JSON olarak logla
	if data, errJ := json.MarshalIndent(product, "", "  "); errJ == nil {
		fmt.Printf("\n--- Ürün Detayı ---\n%s\n", string(data))
	}

	if product.Barcode != barcode {
		t.Errorf("Beklenen barkod %s, gelen %s", barcode, product.Barcode)
	}
}

// TestProductGetMultiple listeleme API'sinden ürün koleksiyonu alır.
func TestProductGetMultiple(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	products, _, err := client.Products.List(ctx, 0, 50)
	if err != nil {
		t.Fatalf("Ürün listesi çekilemedi: %v", err)
	}
	if len(products) == 0 {
		t.Fatalf("Ürün listesi boş döndü")
	}

	t.Logf("Toplam %d ürün listelendi", len(products))
	for i, p := range products[:min(3, len(products))] {
		t.Logf("Ürün %d: %s - %s", i+1, p.Barcode, p.Title)
	}
}

// TestProductDelete virgülle ayrılmış barkod listesini siler ve batch sonuçlarını doğrular.
func TestProductDelete(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if *deleteBarcodesFlag == "" {
		t.Skip("delete parametresi belirtilmedi, test atlandı")
	}

	// Virgülle ayrılmış barkod listesini ayrıştır
	var barcodes []string
	parts := strings.Split(*deleteBarcodesFlag, ",")
	for _, p := range parts {
		bc := strings.TrimSpace(p)
		if bc != "" {
			barcodes = append(barcodes, bc)
		}
	}
	if len(barcodes) == 0 {
		t.Fatal("delete parametresi geçersiz, en az bir barkod belirtmelisiniz")
	}

	fmt.Printf("--- Silinecek %d ürün: %s ---\n", len(barcodes), strings.Join(barcodes, ", "))

	delResp, err := client.Products.Delete(ctx, barcodes)
	if err != nil {
		t.Fatalf("Ürünler silinemedi: %v", err)
	}
	if delResp.BatchRequestID == "" {
		t.Fatalf("BatchRequestID boş döndü")
	}

	fmt.Printf("BatchRequestID (Delete): %s\n", delResp.BatchRequestID)
	fmt.Println("⚠️  Delete için Trendyol batch status API mevcut olmadığından ek doğrulama yapılamıyor.")
}
