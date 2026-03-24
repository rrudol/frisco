# Security Policy

## Supported Versions

This project is currently pre-`1.0.0`. Security fixes are provided for the latest commit on `main`.

## Reporting a Vulnerability

Please do not open public issues for security-sensitive reports.

Use one of the following:

1. GitHub Security Advisories (preferred), if enabled for the repository.
2. Direct private contact with the maintainer via GitHub profile if advisories are unavailable.

Please include:

- A short description of the issue and impact.
- Reproduction steps or proof of concept.
- Affected command/tool (CLI or MCP) and environment details.

I will acknowledge the report as soon as possible and aim to provide an initial assessment within 72 hours.

## Security Notes

- Credentials/tokens are stored locally in `~/.frisco-cli/session.json`.
- Access to that file effectively grants API access in the user context.
- `frisco mcp` uses the same local session; run it only in trusted local environments.
