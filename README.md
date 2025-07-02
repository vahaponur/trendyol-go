# Trendyol Go SDK

> **Önemli Bilgilendirme**  
> Trendyol, **25 Şubat 2025** tarihinde Marketplace API uç noktalarının neredeyse tamamını kapsayan büyük bir versiyon güncellemesi yayınladı.  
> Bu SDK, söz konusu değişiklikleri gözeterek **2 Temmuz 2025** itibarıyla en güncel haliyle yayınlanmıştır.  
> Gelecekte Trendyol'un benzer çapta bir değişiklik yapma ihtimalini göz önünde bulundurarak uç nokta (**endpoint**) ve temel adres (**base URL**) değerleri paket içinde kolaylıkla **override** edilebilecek şekilde tasarlanmıştır.  
> Modeller (struct'lar) ve kimlik doğrulama (Basic Auth) mantığı değişmediği sürece, SDK'yı **yeniden çatallamaya veya dosya düzenlemeye gerek kalmadan** yalnızca `WithEndpointOverrides(...)` opsiyonu ya da çalışma zamanı `client.SetBaseURL()` çağrısıyla kolayca uyarlayabilirsiniz. ([Detaylı bilgi için tıklayın](#api-değiştiyse-nasıl-uyarlanır))

Go dili için Trendyol Marketplace REST API istemcisi.

> **Not**: Bu proje hâlen erken aşamadadır. Şu anda yalnızca bazı ürün uç noktaları test edildi. Diğer servisler eklendikçe örnekler genişletilecektir.

---
## Sandbox/Test Ortamı

> **Sandbox/Test Ortamı**: Trendyol'un test (stage) API'sine erişmek için IP adresinizin önceden Trendyol tarafından **whitelist** edilmesi gerekir; aksi halde 503 hatası alırsınız. Bu nedenle örneklerde `isSandbox=false` (canlı ortam) olarak bırakılmıştır. Test erişimi olanlar `true` geçerek sandbox'u kullanabilir.

```go
// Sandbox (test) ortamı için IP whitelist gerekir
client := trendyol.NewClient("SELLER_ID", "API_KEY", "API_SECRET", true)

// Canlı ortam (IP kısıtı yok)
client := trendyol.NewClient("SELLER_ID", "API_KEY", "API_SECRET", false)
```

---
## Kurulum

```bash
go get github.com/vahaponur/trendyol-go
```

Go modules kullanıyorsanız paketi içe aktarın:

```go
import "github.com/vahaponur/trendyol-go"
```

## Hızlı Başlangıç

```go
package main

import (
    "context"
    "fmt"

    "github.com/vahaponur/trendyol-go"
)

func main() {
    client := trendyol.NewClient("SELLER_ID", "API_KEY", "API_SECRET", false)
    ctx := context.Background()

    // ÖRNEK 1 – Ürün oluşturma (Create)
    newProd := trendyol.Product{
        Barcode:       "ABC-001",
        Title:         "Pamuk Hoodie",
        ProductMainID: "HOOD-001",
        BrandID:       1791,
        CategoryID:    411,
        Quantity:      5,
        StockCode:     "STK-ABC-001",
        CurrencyType:  "TRY",
        ListPrice:     249.90,
        SalePrice:     149.90,
        VATRate:       20,
        Images: []trendyol.ProductImage{{URL: "https://example.com/img.jpg"}},
        Attributes: []trendyol.ProductAttribute{{AttributeID: 1192, AttributeValueID: 10617344}}, // Menşei: TR
    }

    batch, err := client.Products.Create(ctx, []trendyol.Product{newProd})
    if err != nil { panic(err) }
    fmt.Println("BatchRequestID:", batch.BatchRequestID)

    // ÖRNEK 2 – Tekil ürün çekme
    p, err := client.Products.GetByBarcode(ctx, newProd.Barcode)
    if err != nil { panic(err) }
    fmt.Println("Ürün:", p.Title)

    // ÖRNEK 3 – Listeleme
    list, page, err := client.Products.List(ctx, 0, 50)
    if err != nil { panic(err) }
    fmt.Printf("Toplam ürün: %d (sayfa %d/%d)\n", page.TotalElement, page.Page+1, page.TotalPages)
}
```

### Ortam Değişkenleri (Entegrasyon Testleri)

İstemciyi test etmek için `SELLER_ID`, `API_KEY`, `API_SECRET` değerlerini `.env` dosyanıza ekleyin. `env.example` örnek formatı göstermektedir.

```bash
SELLER_ID=123456
API_KEY=YOUR_API_KEY
API_SECRET=YOUR_API_SECRET
```

> **Uyarı**: Gerçek kimlik bilgilerinizi _asla_ repoya göndermeyin.

---

## Desteklenen Servisler

| Servis | Test Edilen Metotlar | Durum |
|--------|---------------------|-------|
| `Products` | `Create`, `GetByBarcode`, `List`, `GetBatchStatus` | ✅ Çalışıyor |
| Diğer tüm servis ve metotlar | | ⚠️ Henüz manuel/entegrasyon testi yapılmadı |

İlerledikçe tablo güncellenecektir.

---

## Yol Haritası

- [x] Ürün oluşturma entegrasyon testi
- [ ] Sipariş servisleri
- [ ] Kargo & finans servisleri
- [ ] Tamamlayıcı örnek kodlar ve dokümantasyon

Katkılar memnuniyetle karşılanır! Forklayıp PR gönderin veya Issue açın.

---


## API Değiştiyse Nasıl Uyarlanır?

```go
// Yeni base URL örneği
client := trendyol.NewClient(
    "SELLER_ID",
    "API_KEY",
    "API_SECRET",
    false,
)

// Trendyol yeni domain'e geçtiyse:
client.SetBaseURL("https://newapi.trendyol.com")

// Sadece tek bir endpoint path'i değiştiyse:
clientWithOverride := trendyol.NewClient(
    "SELLER_ID",
    "API_KEY",
    "API_SECRET",
    false,
    trendyol.WithEndpointOverrides(map[string]string{
        // Anahtar; README'nin "Endpoint Listesi" tablosundaki isimlerden biri
        "CreateTestOrder": "/v2/test/orders/core",
    }),
)
```

* `SetBaseURL(...)` → Tüm isteklerde kullanılan ana domain'i anında değiştirir.
* `WithEndpointOverrides(map[key]template])` → Yalnızca değiştirdiğiniz uç noktalarda geçerlidir; geri kalanlar varsayılan şablonla devam eder.
* `client.GetEndpoints()` → Çalışma zamanında _etkin_ tüm endpoint şablonlarını (override dâhil) görmek için kullanabilirsiniz.

Bu sayede Trendyol ileride yine köklü bir URL değişikliğine giderse, **sadece** yukarıdaki iki satırı güncelleyerek entegrasyonunuzu çalışır halde tutabilirsiniz.

---

## Lisans

MIT © 2025 [@vahaponur](https://github.com/vahaponur)

---