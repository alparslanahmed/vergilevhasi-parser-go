# Test Data

This directory is for storing test PDF files used in testing.

## Usage

To run OCR tests, place a sample Turkish tax plate (Vergi Levhası) PDF file in this directory:

```
testdata/sample_vergi_levhasi.pdf
```

Then update the `expectedTestVKN` constant in `imgconv_test.go` to match the VKN in your test PDF.

## Privacy Notice

**Do not commit real tax plate PDFs to version control.** Use anonymized or synthetic test data only.

The `.gitignore` file is configured to ignore PDF files in this directory.
