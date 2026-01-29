# Test Data

This directory is for storing test PDF files used in testing.

## Usage

To run tests with sample PDFs, place Turkish tax plate (Vergi Levhası) PDF files in this directory.

The parser tests in `parser_test.go` use synthetic text data. For integration testing with real PDFs, you can add sample files here.

## Privacy Notice

**Do not commit real tax plate PDFs to version control.** Use anonymized or synthetic test data only.

The `.gitignore` file is configured to ignore PDF files in this directory.
