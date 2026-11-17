# Changelog

Release highlights for this project. The format follows [Keep a Changelog](https://keepachangelog.com/), and versions follow [Semantic Versioning](https://semver.org/).

## v2.0.3

- **Custom LLM session header**: Each LLM configuration can specify an HTTP header whose value is automatically set to the current session ID. This supports gateways such as opencode that use sessions for prompt caching or sticky routing.
- **Global outbound proxy**: All agents can route target traffic through an HTTP, HTTPS, or SOCKS5 proxy to hide the source IP or use a relay. A connectivity test displays the outbound IP through the proxy.
- **Per-task outbound routing**: When creating a task, choose whether it uses the global proxy or connects directly.
- **Enrichment on confirmation**: Manually confirming a finding automatically enriches it into structured data for report delivery.
- **Report management**: New reports list only confirmed findings by default, including status and CVSS. Report content reuses enriched data.
- **Automatic evidence captions**: Screenshot captions are generated from reproduction steps and remain editable.
- **Finding details improvements**: Status dropdown, section headings, and one-click copying.
- **Traffic filtering fix**: Filtering by task now also applies the task's activity window, keeping traffic from separate tests of the same target apart.
- The System Settings page now uses a responsive two-column layout.

## v2.0.2

- **One-click updates**: Check for and install new GitHub releases from System Settings, with SHA256 verification, atomic replacement, and automatic rollback on failure. Existing data is preserved.
- Release packages include the `start.sh` supervisor script and a systemd service template.

## v2.0.1

- Reproduction steps are automatically linked to the corresponding recorded traffic when saved.

## v2.0.0

- Evidence-driven delivery: Capture reproduction steps as they happen, display findings as step-by-step exploit chains, and link each step to recorded traffic. Generate professional reports in Simplified Chinese, Traditional Chinese, or English.
