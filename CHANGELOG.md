# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2026-01-26

### Changed
- **BREAKING**: Migrated from `ledongthuc/pdf` to `pdfcpu/pdfcpu` v0.11.1 for PDF parsing
  - Uses `pdfcpu`'s `ReadValidateAndOptimize` for safe PDF reading
  - Uses `pdfcpu`'s `ExtractPageContent` for text extraction
  - Added helper functions to parse PDF content streams

### Fixed
- Properly handle escaped parentheses in PDF strings
- Extended hex string decoding to include Latin extended characters (128-255) for Turkish characters like İ, Ş, Ğ, Ö, Ü, Ç

## [1.0.0] - 2026-01-26

### Added
- Initial public release
- PDF parsing for Turkish tax plate (Vergi Levhası) documents
- Extract structured data: name, trade name, address, tax types, activity codes, tax office, VKN, TC ID, start date, historical tax bases
- OCR support (optional) for extracting VKN from barcode images
- Debug mode for inspecting extracted text
- Bilingual documentation (Turkish/English)
- MIT License
