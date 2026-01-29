# vergilevhasi-parser-go

[![CI](https://github.com/alparslanahmed/vergilevhasi-parser-go/actions/workflows/go.yml/badge.svg)](https://github.com/alparslanahmed/vergilevhasi-parser-go/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/alparslanahmed/vergilevhasi-parser-go.svg)](https://pkg.go.dev/github.com/alparslanahmed/vergilevhasi-parser-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/alparslanahmed/vergilevhasi-parser-go)](https://goreportcard.com/report/github.com/alparslanahmed/vergilevhasi-parser-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Türkiye Cumhuriyeti Gelir İdaresi Başkanlığı tarafından PDF formatında verilen/indirilen vergi levhalarından bilgileri yapılandırılmış (structured) olarak çıkaran bir Golang kütüphanesi.

A Go library for extracting structured data from Turkish tax plate (Vergi Levhası) PDF documents.

---

**[Türkçe](#türkçe)** | **[English](#english)**

---

## Türkçe

## Özellikler

Bu kütüphane, vergi levhası PDF'lerinden aşağıdaki bilgileri çıkarabilir:

- **Adı Soyadı** - Kişi adı ve soyadı (şahıs için)
- **Ticaret Ünvanı** - Şirket ticaret ünvanı (şirket için)
- **İş Yeri Adresi** - İşyeri adresi bilgisi
- **Vergi Türü** - Vergi türleri (Gelir Vergisi, KDV, vb.)
- **Faaliyet Kodları ve Adları** - NACE faaliyet kodları ve açıklamaları
- **Vergi Dairesi** - Bağlı olunan vergi dairesi
- **Vergi Kimlik No** - Vergi kimlik numarası
- **TC Kimlik No** - TC kimlik numarası (şahıs için)
- **İşe Başlama Tarihi** - İşe başlama tarihi
- **Geçmiş Matrahlar** - Geçmiş yıllara ait matrah bilgileri

## Kurulum

```bash
go get github.com/alparslanahmed/vergilevhasi-parser-go
```

## Kullanım

### Temel Kullanım

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/alparslanahmed/vergilevhasi-parser-go"
)

func main() {
    // Parser oluştur
    parser := vergilevhasi.NewParser()
    
    // PDF dosyasını parse et
    result, err := parser.ParseFile("vergi-levhasi.pdf")
    if err != nil {
        log.Fatal(err)
    }
    
    // Sonuçları JSON olarak yazdır
    jsonData, _ := json.MarshalIndent(result, "", "  ")
    fmt.Println(string(jsonData))
    
    // Belirli alanları yazdır
    fmt.Printf("Vergi Kimlik No: %s\n", result.VergiKimlikNo)
    fmt.Printf("Vergi Dairesi: %s\n", result.VergiDairesi)
}
```

### io.Reader ile Kullanım

```go
file, err := os.Open("vergi-levhasi.pdf")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

parser := vergilevhasi.NewParser()
result, err := parser.Parse(file)
if err != nil {
    log.Fatal(err)
}
```

### Debug Modu

Debug modunu aktifleştirerek PDF'den çıkarılan ham metni görebilirsiniz:

```go
parser := vergilevhasi.NewParser()
parser.SetDebug(true)
result, err := parser.ParseFile("vergi-levhasi.pdf")
```

## Örnek Çıktı

> **Not:** Aşağıdaki veriler tamamen hayalidir ve yalnızca örnek amaçlıdır.

```json
{
  "adi_soyadi": "Ali Örnek",
  "vergi_kimlik_no": "1234567890",
  "tc_kimlik_no": "11111111110",
  "vergi_dairesi": "Örnek Vergi Dairesi",
  "is_yeri_adresi": "Örnek Mah. Test Cad. No:1, Ankara",
  "ise_baslama_tarihi": "2020-01-15T00:00:00Z",
  "vergi_turu": [
    "Gelir Vergisi",
    "KDV"
  ],
  "faaliyet_kodlari": [
    {
      "kod": "4711",
      "ad": "Gıda, içecek ve tütün ağırlıklı olan mağazalarda perakende ticaret"
    }
  ],
  "gecmis_matrahlar": [
    {
      "yil": 2020,
      "tutar": 100000.0
    },
    {
      "yil": 2021,
      "tutar": 150000.0
    }
  ]
}
```

## API Dokümantasyonu

### `NewParser() *Parser`

Yeni bir parser örneği oluşturur.

### `(*Parser) SetDebug(debug bool)`

Debug modunu aktif/pasif eder. Debug modunda PDF'den çıkarılan ham metin konsola yazdırılır.

### `(*Parser) ParseFile(filepath string) (*VergiLevhasi, error)`

Belirtilen yoldaki PDF dosyasını parse eder ve yapılandırılmış veriyi döndürür.

### `(*Parser) Parse(reader io.ReadSeeker) (*VergiLevhasi, error)`

io.ReadSeeker'dan PDF dosyasını parse eder ve yapılandırılmış veriyi döndürür.

## Veri Yapısı

### `VergiLevhasi`

```go
type VergiLevhasi struct {
    AdiSoyadi        string      // Adı Soyadı
    TicaretUnvani    string      // Ticaret Ünvanı
    IsYeriAdresi     string      // İş Yeri Adresi
    VergiTuru        []string    // Vergi Türleri
    FaaliyetKodlari  []Faaliyet  // Faaliyet Kodları
    VergiDairesi     string      // Vergi Dairesi
    VergiKimlikNo    string      // Vergi Kimlik No
    TCKimlikNo       string      // TC Kimlik No
    IseBaslamaTarihi *time.Time  // İşe Başlama Tarihi
    GecmisMatra      []Matrah    // Geçmiş Matrahlar
}
```

### `Faaliyet`

```go
type Faaliyet struct {
    Kod string // Faaliyet kodu (örn: "4711")
    Ad  string // Faaliyet açıklaması
}
```

### `Matrah`

```go
type Matrah struct {
    Yil   int     // Yıl
    Donem string  // Dönem (varsa)
    Tutar float64 // Tutar
    Tur   string  // Matrah türü (varsa)
}
```

## Test

```bash
# Run basic tests
go test -v

# Run tests with coverage
go test -v -cover
```

## Örnek Uygulama

Proje içinde örnek uygulama bulunmaktadır:

```bash
# Örneği çalıştır
go run example/main.go path/to/vergi-levhasi.pdf
```

## Bağımlılıklar

- [github.com/pdfcpu/pdfcpu](https://github.com/pdfcpu/pdfcpu) - PDF işleme ve metin çıkarma kütüphanesi
- [github.com/makiuchi-d/gozxing](https://github.com/makiuchi-d/gozxing) - Barkod tarama kütüphanesi (VKN çıkarma için)

## Lisans

MIT License - Detaylar için [LICENSE](LICENSE) dosyasına bakın.

## Katkıda Bulunma

Katkılarınızı bekliyoruz! Detaylar için [CONTRIBUTING.md](CONTRIBUTING.md) dosyasına bakın.

**Önemli:** Gerçek vergi levhası PDF'lerini veya kişisel verileri repository'ye eklemeyin. Test verileri için yalnızca hayali/sahte veriler kullanın.

## Not

Bu kütüphane, Gelir İdaresi Başkanlığı'nın PDF formatındaki vergi levhalarından bilgi çıkarmak için regex tabanlı bir yaklaşım kullanır. PDF formatı değişiklik gösterebileceğinden, tüm vergi levhası formatları ile tam uyumlu olmayabilir. Farklı formatlarla karşılaşırsanız, lütfen bir issue açın veya pull request gönderin.

### Bilinen Sınırlamalar

- **Vergi Kimlik No (VKN)**: Bazı GİB PDF'lerinde VKN, barkod fontları ile veya barkod görseli içinde render edildiğinden metin olarak çıkarılamayabilir. Bu durumlarda VKN alanı boş kalabilir. VKN'yi çıkarmak için OCR kullanabilirsiniz (aşağıya bakın).
- **Türkçe Karakter Sorunları**: PDF'den metin çıkarma sırasında bazı Türkçe karakterler (Ö, Ü, İ, vb.) eksik veya hatalı çıkabilir. Bu, PDF kütüphanesinin sınırlamalarından kaynaklanmaktadır.

## OCR ile Vergi Kimlik No Çıkarma

Bazı GİB PDF'lerinde VKN barkod görseli olarak render edildiğinden normal metin çıkarma ile elde edilemez. Bu durumda OCR (Optik Karakter Tanıma) kullanabilirsiniz.

### Sıfır Bağımlılık (Zero Dependencies)

OCR özelliği tamamen saf Go ile yazılmıştır ve **hiçbir harici bağımlılık gerektirmez**:
- ❌ ONNX Runtime gerekmez
- ❌ TensorFlow gerekmez
- ❌ Tesseract gerekmez
- ❌ Harici araç gerekmez

### Kullanım

```go
package main

import (
    "fmt"
    "log"

    vergilevhasi "github.com/alparslanahmed/vergilevhasi-parser-go"
)

func main() {
    // OCR parser oluştur (hiçbir parametre gerekmez!)
    parser, err := vergilevhasi.NewOCRParser()
    if err != nil {
        log.Fatal(err)
    }
    defer parser.Close()

    // Görsel dosyasından VKN çıkar
    vkn, err := parser.ExtractVKNFromImage("vergi-levhasi.png")
    if err != nil {
        log.Printf("OCR hatası: %v", err), err)
    } else {
        fmt.Printf("Vergi Kimlik No: %s\n", vkn)
    }
}
```

### Build

```bash
# Normal build (OCR is included by default)
go build .

# Örneği çalıştır
go run example/main.go path/to/vergi-levhasi.pdf
```

### Desteklenen Formatlar

- **PNG**: Doğrudan işlenir
- **JPG/JPEG**: Doğrudan işlenir
- **PDF**: Otomatik olarak görsele çevrilir

### En İyi Sonuçlar İçin

1. **Görsel kalitesi**: Yüksek çözünürlüklü, net görsel kullanın
2. **Kontrast**: Siyah metin, beyaz arka plan tercih edilir
3. **Kırpma**: Sadece VKN numarasını içeren bölgeyi kırpın
4. **Eğim**: Görsel düz olmalı, eğik olmamalı

### Nasıl Çalışır

OCR modülü şu adımları izler:

1. **Gri tonlamaya çevirme**: Renk bilgisi kaldırılır
2. **Adaptif binarizasyon**: Görsel siyah-beyaza dönüştürülür
3. **Bağlı bileşen analizi**: Her rakam ayrı ayrı tespit edilir
4. **Özellik çıkarma**: Her rakam için geometrik özellikler hesaplanır
5. **Sınıflandırma**: Özellikler kullanılarak rakam tanınır
6. **VKN deseni arama**: 10 haneli numara bulunur

**Not:** Bu saf Go implementasyonu, MNIST tabanlı sinir ağı modellerine göre daha düşük doğruluğa sahip olabilir. En iyi sonuçlar için görsel kalitesine dikkat edin.

**Not:** OCR özelliği opsiyoneldir. Temel PDF metin çıkarma için OCR gerekli değildir.

---

## English

### Features

This library can extract the following information from tax plate PDFs:

- **Full Name (Adı Soyadı)** - For individual taxpayers
- **Trade Name (Ticaret Ünvanı)** - For companies
- **Business Address (İş Yeri Adresi)**
- **Tax Types (Vergi Türü)** - Income Tax, VAT, etc.
- **Activity Codes (Faaliyet Kodları)** - NACE activity codes and descriptions
- **Tax Office (Vergi Dairesi)**
- **Tax ID Number (Vergi Kimlik No - VKN)**
- **Turkish ID Number (TC Kimlik No)** - For individuals
- **Business Start Date (İşe Başlama Tarihi)**
- **Historical Tax Bases (Geçmiş Matrahlar)**

### Installation

```bash
go get github.com/alparslanahmed/vergilevhasi-parser-go
```

### Quick Start

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"

    vergilevhasi "github.com/alparslanahmed/vergilevhasi-parser-go"
)

func main() {
    parser := vergilevhasi.NewParser()
    
    result, err := parser.ParseFile("vergi-levhasi.pdf")
    if err != nil {
        log.Fatal(err)
    }
    
    jsonData, _ := json.MarshalIndent(result, "", "  ")
    fmt.Println(string(jsonData))
}
```

### OCR Support

Some GİB PDFs encode the VKN as a barcode image rather than text. For these cases, use the OCR parser:

```go
parser, _ := vergilevhasi.NewOCRParser()
defer parser.Close()
vkn, err := parser.ExtractVKNFromImage("vergi-levhasi.png")
// or from PDF bytes:
// vkn, err := parser.ExtractVKNFromPDFBytes(pdfData)
```

See the [Türkçe section](#ocr-ile-vergi-kimlik-no-çıkarma) for detailed OCR documentation.
