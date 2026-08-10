# Contributing

This is a clean-room implementation. Do not copy code, data structures or
line-by-line algorithms from LinBPQ or other reference implementations.

Protocol changes must include:

1. a public specification reference in `docs/protocol-sources.md`;
2. tests for valid and malformed inputs;
3. a note where the standard is ambiguous;
4. no runtime dependency on the local `Zródał/` directory.

Run `go test ./...`, `go vet ./...` and `go build ./...` before submitting a
change.

