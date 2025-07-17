.PHONY: help upload get-single get-multiple delete integration

# Ortak go test parametreleri
GO_TEST = go test ./integration -tags=integration -v -count=1

# -----------------------------------------------------------------------------
#  Yardım ve toplu çalışma hedefleri
# -----------------------------------------------------------------------------
help:
	@echo "Entegrasyon testleri için kısayollar:"
	@echo "  make upload                -> TestProductUpload"
	@echo "  make get-single BARCODE=...-> TestProductGetSingle (tek barkod)"
	@echo "  make get-multiple          -> TestProductGetMultiple"
	@echo "  make delete DELETE=...     -> TestProductDelete (virgüllü barkod listesi)"
	@echo "  make integration           -> integration klasöründeki tüm testler"
	@echo ""
	@echo "Örnek: make delete DELETE=ABC123,XYZ456"

# Tüm integration testlerini çalıştırır
integration:
	$(GO_TEST)

# -----------------------------------------------------------------------------
#  Tekil test hedefleri (integration/product_test.go)
# -----------------------------------------------------------------------------

upload:
	$(GO_TEST) -run ^TestProductUpload$$

get-single:
	$(GO_TEST) -run ^TestProductGetSingle$$ -args -barcode=$(BARCODE)

get-multiple:
	$(GO_TEST) -run ^TestProductGetMultiple$$

# Silme testi, virgülle ayrılmış barkod listesi DELETE değişkeni ile verilir
# Örnek: make delete DELETE=ABC,DEF,XYZ
delete:
	$(GO_TEST) -run ^TestProductDelete$$ -args -delete=$(DELETE) 