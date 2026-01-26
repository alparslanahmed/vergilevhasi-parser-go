# vergilevhasi-parser-go

Türkiye Cumhuriyeti Gelir İdaresi Başkanlığı tarafından PDF formatında verilen/indirilen vergi levhalarından bilgileri yapılandırılmış (structured) olarak çıkaran bir Golang kütüphanesi.

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

```json
{
  "adi_soyadi": "Ahmet Yılmaz",
  "vergi_kimlik_no": "1234567890",
  "tc_kimlik_no": "12345678901",
  "vergi_dairesi": "İstanbul Vergi Dairesi",
  "is_yeri_adresi": "Kadıköy Mahallesi, İstanbul",
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
go test -v
```

## Örnek Uygulama

Proje içinde örnek bir uygulama bulunmaktadır:

```bash
go run example/main.go path/to/vergi-levhasi.pdf
```

## Bağımlılıklar

- [github.com/unidoc/unipdf/v3](https://github.com/unidoc/unipdf) - PDF işleme kütüphanesi

## Lisans

MIT

## Katkıda Bulunma

Pull request'ler kabul edilmektedir. Büyük değişiklikler için lütfen önce bir issue açarak ne değiştirmek istediğinizi tartışın.

## Not

Bu kütüphane, Gelir İdaresi Başkanlığı'nın PDF formatındaki vergi levhalarından bilgi çıkarmak için regex tabanlı bir yaklaşım kullanır. PDF formatı değişiklik gösterebileceğinden, tüm vergi levhası formatları ile tam uyumlu olmayabilir. Farklı formatlarla karşılaşırsanız, lütfen bir issue açın veya pull request gönderin.
