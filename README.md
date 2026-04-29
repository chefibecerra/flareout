# FlareOut

FlareOut is an open-source CLI/TUI tool for managing Cloudflare DNS proxy state. Born from the November 2025 Cloudflare outage, it gives operators a safe, auditable way to toggle the Cloudflare proxy flag on DNS records — with snapshot-before-change, dry-run preview, and a structured audit log — so that disabling or re-enabling the proxy during an incident does not become a second incident.

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

## Status

Pre-alpha — foundation phase. No features are implemented yet. This repository contains the project skeleton only.

## Install

TBD

## Usage

TBD

## Development

TBD

## Token Scope

FlareOut reads your Cloudflare API token from the `CLOUDFLARE_API_TOKEN` environment variable. The token requires at minimum **DNS:Read** scope to list zones and records, and **DNS:Edit** scope for any write operation.

**Important**: Cloudflare does not expose the granted scopes in the token-verification API response. FlareOut cannot verify that your token has the DNS:Edit scope before you attempt a write operation. A non-suppressible warning is printed at startup to remind you of this limitation. If your token is missing a required scope, the first failing API call will surface a permission error.

## License

Apache 2.0 — see the [LICENSE](LICENSE) file for the full text.
