# Contributing to vergilevhasi-parser-go

Thank you for considering contributing to this project! Here are some guidelines to help you get started.

## How to Contribute

### Reporting Bugs

If you find a bug, please open an issue on GitHub with:
- A clear description of the problem
- Steps to reproduce the issue
- Expected behavior vs. actual behavior
- Go version and OS information

**Important:** Do NOT include real tax plate PDFs or personal data in bug reports. If the issue is format-related, describe the structure without sharing actual data.

### Suggesting Features

Feature suggestions are welcome! Please open an issue describing:
- The feature you'd like to see
- Why it would be useful
- Any implementation ideas you have

### Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`go test -v`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Development Guidelines

### Code Style

- Follow standard Go conventions and formatting (`go fmt`)
- Add comments for exported functions and types
- Keep functions focused and modular

### Testing

- Add tests for new features
- Use dummy/fictional data in tests (never real personal data)
- Run `go test -v` before submitting PRs

### Privacy

**This is critical:** Never commit real personal data to this repository.

- Test PDFs should use synthetic/dummy data
- Example code should use obviously fictional names and numbers
- The `testdata/` directory is gitignored for PDFs

### Turkish Character Handling

When adding new patterns or text matching:
- Consider both ASCII and Turkish character variants (e.g., `İ` vs `I`, `ş` vs `s`)
- Test with various encodings that may appear in PDFs

## Running Tests

```bash
# Run all tests
go test -v

# Run tests with coverage
go test -v -cover
```

## Project Structure

```
├── vergilevhasi.go    # Core data structures
├── parser.go          # PDF text parsing logic
├── ocr.go             # OCR functionality for barcode/image extraction
├── parser_test.go     # Unit tests
├── example/           # Example application
│   └── main.go        # Full example with OCR
└── testdata/          # Test files (gitignored)
```

## Questions?

Feel free to open an issue for any questions about contributing.
