# Security Policy

## Reporting a vulnerability

**Do not open a public issue.** Instead, email **enriquefft@404tf.com** with:

- A description of the vulnerability
- Steps to reproduce
- Any relevant logs or screenshots

We will acknowledge your report within **48 hours** and aim to provide a fix or mitigation timeline within 7 days.

## Scope

The following are considered security issues:

- Authentication or authorization bypass (e.g., circumventing the sender allowlist)
- Session isolation leaks (cross-sender context exposure)
- Rate limiting bypass
- Webhook signature validation bypass
- Information disclosure through error messages or logs
- Injection attacks via message content

The following are **not** security issues (please open a regular issue instead):

- Denial of service through excessive API calls (rate limiting is best-effort)
- Issues requiring physical access to the host
- Social engineering attacks
- Vulnerabilities in upstream dependencies (report those upstream, but let us know)

## Built-in security features

This project includes several security controls by default:

- **Sender allowlist** — only authorized phone numbers can interact with the agent
- **Per-sender rate limiting** — fixed-window token bucket prevents abuse
- **Role tagging** — messages carry `[role: <role>]` for capability enforcement
- **Session isolation** — each sender gets a separate OpenClaw session, preventing cross-sender context leakage
