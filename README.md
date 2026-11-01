<h1 align="center">OpenTaskKit</h1>

<p align="center"><strong>A self-hosted, local-first task manager.</strong><br>
Plan your day, keep projects in order, set reminders and share lists with your family or your team. Works on your phone and your computer, even offline. No monthly plan per person, and your tasks stay with you.</p>

<p align="center">
  <a href="LICENSE"><img alt="License: AGPL-3.0" src="https://img.shields.io/badge/license-AGPL--3.0-blue.svg"></a>
  <a href="https://github.com/nufuturepro/OpenTaskKit/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/nufuturepro/OpenTaskKit/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/nufuturepro/OpenTaskKit/releases"><img alt="Latest release" src="https://img.shields.io/github/v/release/nufuturepro/OpenTaskKit?include_prereleases&sort=semver"></a>
  <a href="https://github.com/nufuturepro/OpenTaskKit/pkgs/container/opentaskkit"><img alt="Container image" src="https://img.shields.io/badge/ghcr.io-nufuturepro%2Fopentaskkit-2496ED?logo=docker&logoColor=white"></a>
  <img alt="Docker image size" src="https://img.shields.io/badge/image-%3C%2025%20MB-success">
</p>

---

## What this is

OpenTaskKit is a **distribution of [OpenTodo](https://github.com/pixari/opentodo)**, with some in-house updates:

- **Upstream:** all features and bug fixes come from [pixari/OpenTodo](https://github.com/pixari/opentodo), a great task manager built in the open with specs. Use its [demo](https://demo.open-todo.app), its [documentation](https://open-todo.app/docs/) and its [issue tracker](https://github.com/pixari/opentodo/issues) — that is where the project lives, and general bug fixes belong there.
- **This repository** carries the OpenTaskKit branding and will carry this distribution's own defaults and features. It merges upstream releases regularly (see [FORKING.md](FORKING.md) for the exact workflow, branch scheme and the list of files the brand touches).
- Generic bug fixes found here are sent upstream first, so everyone benefits and the two stay close.

OpenTaskKit is licensed **AGPL-3.0**, exactly like upstream. It is not affiliated with or endorsed by the upstream author; if OpenTaskKit saves you a subscription, consider [supporting upstream](https://buymeacoffee.com/pixari) and starring [pixari/OpenTodo](https://github.com/pixari/opentodo).

## Quick start

On your machine or server (no configuration needed):

```bash
docker run -d -p 8080:8080 -v opentodo-data:/data ghcr.io/nufuturepro/opentaskkit:stable
```

Open <http://localhost:8080>; the first visit creates your account. The data volume keeps the upstream name (`opentodo-data`) and the archive format, so data moves freely between OpenTaskKit and OpenTodo.

Or with Docker Compose:

```bash
git clone https://github.com/nufuturepro/OpenTaskKit.git && cd OpenTaskKit
docker compose up -d
```

Behind a reverse proxy, set `OPENTODO_BASE_URL=https://todo.example.com` (needed for secure cookies, passkeys and web push). All environment variables keep their upstream `OPENTODO_` names.

## Try it in 30 seconds

No install: open the [upstream demo](https://demo.open-todo.app) and press **Try the demo**. You get a private copy with sample data, deleted after a few hours. The demo runs upstream OpenTodo; OpenTaskKit behaves the same apart from the name.

## Why it is good

- **No subscription, no lock-in.** Free software under the AGPL. Nobody can paywall your task list or shut it down with the company.
- **Works offline, on every device.** Each phone and computer keeps a full copy of your data (local-first), so the app works with no connection at all. The server syncs, backs up and shares; it is never in the way of an edit.
- **Your data, one folder.** Everything lives in one `/data` volume around a single SQLite file. Export to JSON at any time, import it (or a CSV, or another app's export) on any instance, and schedule encrypted backups to disk, S3 or WebDAV.
- **Tiny footprint.** One container, one static binary of about 23 MB with the web app inside, around 20 MB of memory when idle. Runs on a Raspberry Pi or the cheapest VPS.
- **Private by design.** No telemetry, no analytics, no phone-home, no third-party scripts or fonts.
- **Keyboard-first, spec-driven, AI-optional.** Vim keybindings, quick add in plain language in six languages, time blocking, shared projects, CalDAV, a REST API and CLI, passkeys, two-factor, single sign-on, optional end-to-end encryption, and bring-your-own-AI that is off by default.

The full feature list, guides and the reference live in the [upstream documentation](https://open-todo.app/docs/) and inside the app under **Help** (`g` then `?`); they describe OpenTaskKit accurately apart from the name on a few pages.

## Configuration

Every setting is an optional environment variable with a safe default; an invalid value stops startup with a message naming the variable. The container image sets `OPENTODO_ADDR=:8080` and `OPENTODO_DATA_DIR=/data`. See [`.env.example`](.env.example) for a commented template.

| Variable | Default | Purpose |
| --- | --- | --- |
| `OPENTODO_ADDR` | `:8080` | Listen address |
| `OPENTODO_DATA_DIR` | `./data` (binary), `/data` (image) | Directory for the SQLite database, secrets, the VAPID key and local backups |
| `OPENTODO_BASE_URL` | unset (inferred from requests) | Public URL. With https: `Secure` cookies, `Strict-Transport-Security`, passkeys and web push; also used for strict origin checks (the request must be addressed to its host) and in invitation, reset and CalDAV links |
| `OPENTODO_SETUP_TOKEN` | unset | When set (16+ characters), creating the first account requires this token from every address, so a fresh public server cannot be claimed by a stranger. No effect once the owner exists. The Coolify template generates one |
| `OPENTODO_SETUP_CODE` | `off` | `required` makes a first registration from another machine, while no owner and no setup token exist, carry the one-time [setup code](docs/self-hosting/docker.md#the-setup-code) printed in the server log. Off, the first visitor becomes the owner: finish setup right after starting a public server |
| `OPENTODO_ALLOW_SIGNUP` | `false` | Let anyone create an account after the owner exists. Prefer [invitations](#adding-people) |
| `OPENTODO_SESSION_DAYS` | `90` | Browser session lifetime in days |
| `OPENTODO_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. The startup line shows the level in use (`log_level=`); `debug` adds one line per API request (method, path, status, duration) |
| `OPENTODO_IMPORT_MAX_MB` | `20` | Largest [import](docs/import.md) file, 1–200 |
| `OPENTODO_ATTACHMENTS_MAX_FILE_MB` | `25` | Optional preset for the largest [attachment](docs/attachments.md), 1–2048; applied once, then Settings decides |
| `OPENTODO_ATTACHMENTS_QUOTA_MB` | `1024` | Optional preset for each account's attachment total, 0 (unlimited)–1048576; applied once |
| `OPENTODO_TRASH_RETENTION_DAYS` | `30` | Days a deleted task or project stays in the Trash before it is purged, 0–3650; `0` never purges automatically. See [Purge](docs/sync.md#purge) |
| `OPENTODO_DEMO` | unset (off) | `true` turns a dedicated instance into a [public demo](docs/self-hosting/demo.md): a private, seeded sandbox per visitor, deleted after 2 h of inactivity, after 24 h and every night at 03:00 UTC; abusable features refused with 403 `demo_restricted` |
| `OPENTODO_DEMO_MAX_SANDBOXES`, `OPENTODO_DEMO_MAX_DB_MB` | `500`, `1024` | Optional demo tuning: live sandbox cap and the database size above which no new sandbox is created |
| `OPENTODO_TRUSTED_PROXIES` | loopback and private networks | Optional: the proxies whose `X-Forwarded-For`, `X-Forwarded-Host` and `X-Forwarded-Proto` are believed, for rate limits, the audit log and the setup code (IPs, CIDRs, `private` for the default ranges, `cloudflare` for Cloudflare's edges plus the default). A list replaces the default; see [Behind a reverse proxy](docs/self-hosting/docker.md#behind-a-reverse-proxy) |
| `OPENTODO_PUSH` | `on` | `off` disables web push for [reminders](docs/reminders.md): no requests to push services; reminders still show in the app |
| `OPENTODO_WEBHOOKS_ALLOW_PRIVATE` | unset | `true`/`false` presets the owner's "allow private network targets" [webhook](docs/webhooks.md#private-network-protection) policy on first start; afterwards Settings decides |
| `OPENTODO_MCP` | unset (off) | `true` presets the [AI assistants (MCP)](docs/mcp.md) endpoint on first start; afterwards Settings → Integrations → AI assistants decides |
| `OPENTODO_MCP_WRITES` | unset (off) | `true` presets "Assistants may make changes" on first start; each user must still allow it and use an `agent` token. Afterwards Settings → Integrations → AI assistants decides |
| `OPENTODO_CALDAV` | unset (off) | `true` presets the [CalDAV](docs/caldav.md) endpoint on first start; afterwards Settings → Integrations → Calendar apps decides |
| `OPENTODO_OIDC_ISSUER`, `_CLIENT_ID`, `_CLIENT_SECRET`, `_DISPLAY_NAME`, `_SSO_ONLY` | unset | Optional [single sign-on](docs/self-hosting/sso.md) settings; each one set overrides and locks that field in Settings → Access policies → Single sign-on. The issuer must be https |
| `OPENTODO_SMTP_HOST`, `_SECURITY`, `_USER`, `_PASSWORD` (or `_PASSWORD_FILE`), `_FROM` | unset (off) | Optional [outgoing email](docs/self-hosting/outgoing-email.md) for password resets, sign-up confirmations and invitations; `HOST` and `FROM` together; each one set overrides and locks that field in Settings → Server → Outgoing email |
| `OPENTODO_STT_URL`, `_MODEL`, `_KEY_FILE` | unset (off) | Optional [speech-to-text service](docs/self-hosting/speech-to-text.md) for dictation (OpenAI-compatible `/v1/audio/transcriptions`); a set variable makes its field read-only in Settings |
| `OPENTODO_EMAIL_ADDRESS`, `_IMAP_HOST`, `_IMAP_USER`, `_IMAP_PASSWORD` (or `_IMAP_PASSWORD_FILE`) | unset (off) | [Email to task](docs/email.md): all four together turn it on; setting only some stops startup |
| `OPENTODO_EMAIL_IMAP_MAILBOX`, `_POLL_SECONDS`, `_PROCESSED` | `INBOX`, `60`, `move:OpenTodo/Processed` | Email to task tuning (minimum poll 15 s; `_PROCESSED` may be `delete`) |
| `OPENTODO_BACKUP_*` | unset | Optional presets for [scheduled backups](docs/backups.md#environment-presets) (`DESTINATION`, `SCHEDULE`, `TIME`, `WEEKDAY`, `KEEP`, `MAX_AGE_DAYS`, `COMPRESS`, `LOCAL_DIR`, `S3_*`, `WEBDAV_*`, `PASSPHRASE`/`PASSPHRASE_FILE`), applied once; Settings → Backups wins afterwards |
| `OPENTODO_HOST_URL`, `_TOKEN`, `_OPERATOR_ADDR`, `_IDLE_MINUTES`, `_MAX_OPEN`, `_LIMITS` | unset (off) | [Host mode](docs/self-hosting/host-mode.md) for operators of a hosted service: many tenants in one process, chosen by host name. Not needed to self-host |

Environment variables keep their upstream `OPENTODO_` names so every guide, template and deployment written for OpenTodo works unchanged. The full list is in the [upstream configuration reference](https://open-todo.app/docs/self-hosting/configuration/) and in [`.env.example`](.env.example).

## Adding people

Closed sign-up by default: the owner invites people with a single-use invitation link, transfers ownership, deactivates accounts and can require two-factor for everyone. Shared projects have viewer, editor and admin roles. The guides are upstream's: [Adding people](https://open-todo.app/docs/self-hosting/adding-people/) and [Sharing](https://open-todo.app/docs/sharing/).

## Development

Requirements: Go (see `go.mod` for the exact toolchain), Node 22+, GNU make.

```bash
make dev    # API on :8080 (go run) + Vite on :5173 with proxy and hot reload
make check  # vet, staticcheck, typecheck, lint, Go and web tests, i18n, docs, OpenSpec
make build  # production binary in bin/opentodo with the web app embedded
```

Contributing to OpenTaskKit: generic bug fixes should go upstream to [pixari/OpenTodo](https://github.com/pixari/opentodo) first (see its [CONTRIBUTING.md](https://github.com/pixari/opentodo/blob/main/CONTRIBUTING.md); bug fixes need no spec); they flow back here with the next sync. Distribution-specific work (branding, defaults, own features) belongs in this repository. The mechanics — remotes, sync script, brand overlay, versioning — are in [FORKING.md](FORKING.md).

## Privacy

The web app makes no network requests except to your own server: no analytics, crash reporters, update pings or third-party fonts. The server makes outbound requests only for features you configure:

- web push to the push service a device's browser chose (Google, Mozilla, Apple), end-to-end encrypted and carrying only ids unless the device opts in to titles (`OPENTODO_PUSH=off` turns push off);
- webhook URLs you add;
- backup destinations (S3 or WebDAV) you configure;
- the single sign-on issuer you configure;
- the IMAP mailbox you configure for email to task;
- the SMTP server you configure for [outgoing email](docs/self-hosting/outgoing-email.md);
- the GitHub or GitLab host people connect for [issue import](docs/issue-import.md), with their token sent only to that host (private network addresses only if you allow them);
- the calendar addresses people add to show [other calendars](docs/other-calendars.md) beside their tasks (private network addresses only if you allow them);
- the [speech-to-text service](docs/self-hosting/speech-to-text.md) you connect for dictation, only while someone who chose it dictates a phrase;
- the model provider you enable for the [in-app assistant](docs/assistant.md) (off by default), only after a person has accepted what is sent and only while they wait for an answer.

The CalDAV and MCP endpoints only answer requests; they never call out. The audit log records security events (sign-ins, tokens, invitations, exports), never task content, and stores client addresses only if the owner turns that on; see [SECURITY.md](SECURITY.md#what-is-logged). With optional [end-to-end encryption](docs/e2ee.md) the server holds only ciphertext for task content.

## Contributing

Issues and pull requests are welcome, and so are questions on [Discord](https://discord.gg/Jmj7WuRQ). Please read [CONTRIBUTING.md](CONTRIBUTING.md): changes that alter behaviour go through an OpenSpec change first. Security issues go to the address in [SECURITY.md](SECURITY.md).

## License

[AGPL-3.0](LICENSE). OpenTaskKit is a modified distribution of [OpenTodo](https://github.com/pixari/opentodo) by Raffaele Pizzari, used under the AGPL-3.0. Modifications are published in this repository.
