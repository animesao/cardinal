<!-- dck-version:start -->
**Documentation version:** `1.24.15`
**Project release:** `v1.24.15`
<!-- dck-version:end -->

<p align="center">
  <img src="img/dck.png" alt="dck logo" width="120">
</p>

# Contributors

Thank you to everyone who helps improve **dck**.

## Maintainers

- [animesao](https://github.com/animesao) — project maintainer and primary author.

## Automation

- `github-actions[bot]` — automated versioning and release workflow updates.

## How to contribute

Contributions are welcome:

1. Fork the repository and create a focused branch.
2. Make the smallest change that solves the problem.
3. Add or update tests for code changes.
4. Run the local checks before opening a pull request:

   ```bash
   gofmt -w $(git ls-files '*.go')
   go test ./... -count=1
   go vet ./...
   golangci-lint run ./...
   ```

5. Update the relevant English and Russian documentation when behavior or commands change.
6. Open a pull request with a clear description of the change and its verification.

Please do not add a person to this file without their permission. New contributors can be added after a merged contribution or by request.
