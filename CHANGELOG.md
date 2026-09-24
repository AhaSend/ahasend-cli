# Changelog

All notable changes to the AhaSend CLI are recorded in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- **Breaking:** A command that fails because of an API error now exits with a
  nonzero code (1) in `--output json` mode, the same as in the other output
  formats. Before, JSON mode exited with 0. The API's error response is still
  printed to stdout unchanged. Scripts that checked the JSON body to detect
  errors can now rely on the exit code.

### Fixed

- `subaccounts create` and `subaccounts update` now accept a domain for
  `--website`, such as `example.com`, which is the format the API requires.
  A URL with only a scheme and host, such as `https://example.com`, is also
  accepted and sent as `example.com`. URLs with a path, port, query, or
  credentials are rejected.
- SMTP credential output now lists port 587 (STARTTLS, recommended) and ports
  25 and 2525. It no longer shows port 465, which AhaSend does not support.

## Earlier releases

Notes for v0.2.0 and earlier are on the
[GitHub releases page](https://github.com/AhaSend/ahasend-cli/releases).

[Unreleased]: https://github.com/AhaSend/ahasend-cli/compare/v0.2.0...HEAD
