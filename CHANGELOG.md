# Changelog

All notable changes to the AhaSend CLI are recorded in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.5.0] - 2026-11-11

This release adds `stats emails`, for email counts of any period, and names
the statistics scopes `statistics:read`.

### Added

- [`stats emails`](USAGE.mdx#ahasend-stats-emails) shows email counts of any
  period in the account's history, beyond the message metadata retention
  period, by hour, day, week or month: received, delivered, bounced,
  suppressed, complaints, deferred, opened and clicked. `--breakdown domain`
  shows the counts of each domain, and the command fetches every page of
  domains. The CLI uses AhaSend Go SDK v0.7.0.
- The Chocolatey package names its copyright holder, AhaSend B.V.

### Changed

- The statistics scopes are `statistics:read:all` and
  `statistics:read:{domain}`, which also cover `stats emails`. The help and
  the shell completion of `apikeys create` and `update` and of
  `subaccounts api-keys create` and `update` name them, and so do the
  examples of the `apikeys` commands. These commands still accept
  `statistics-transactional:read:all` and
  `statistics-transactional:read:{domain}`, which give the same access, and
  send each scope as it is given. One scope given under both names is
  refused as given more than once.

### Fixed

- The Chocolatey package page holds no email address, which Chocolatey's
  automated validation refuses. Its usage example points to
  `ahasend messages send --help`.

## [0.4.0] - 2026-11-09

This release moves the CLI to AhaSend Go SDK v0.6.0. It adds segments,
segment audiences for campaigns, AhaML designs and the contact webhook events.
Read the breaking changes before you upgrade scripts.

### Breaking changes

- `templates get` shows a template's working copy, the draft while it has
  one, instead of its published copy, so a change with `templates update`
  builds on what it shows. `templates get --published` shows the published
  copy, the one sends use.
- `templates draft get` is removed, as the API no longer reads a draft on its
  own: `templates get` shows it. `templates draft` holds `discard` alone.
- The CSV output of `messages send` has the column `message_id_header` after
  `id`, and that of `campaigns recipients` the columns `segment_id` and
  `segment_name` after `list_name`.
- The CSV output of a template, a template version, a campaign and a
  campaign template, in the `templates`, `templates versions`, `campaigns`
  and `campaigns templates` commands, has the columns `markup` and
  `markup_error` after `mjml`. The single views of a template, a campaign and
  a campaign template end with a `markup_warnings` column.
- The `variables` value of a template and a template version names each
  variable's kind, so `items (required)` reads
  `items (list of name/price; required)` for a list.
- The CSV output of a campaign, in the `campaigns` commands that print one,
  `campaigns send` included, has the columns `segment_ids`,
  `exclude_list_ids` and `exclude_segment_ids` after `list_ids`.
- The limit of 50 lists of `campaigns create` and `campaigns update` counts
  the excluded lists too, and more fail with
  `--list-id and --exclude-list-id take at most 50 lists in all, got <n>`
  instead of `--list-id takes at most 50 lists, got <n>`.
- The CSV output of a webhook, in `webhooks list`, `get`, `create` and
  `update`, has the column `origins` after `events`.
- The event flags of `webhooks update` name the webhook's whole set of 21
  events, so an event they do not name, such as a contact event turned on in
  the dashboard, is turned off.

### Added

- The Chocolatey package has the AhaSend icon.
- `webhooks listen` prints on stderr the warnings the stream sends when it
  connects, such as that the API key does not have the read scope of an
  event family the webhook turns on, so those events are not sent.
- `messages send` that makes one request shows each message's Message-ID
  header, which `messages list --message-id` takes, after its ID.
- `messages get` shows the report message ID of a delivery attempt that was
  bounced out of band: a Report Message ID column in table output and a
  `Report Message ID:` line in plain output.
- `campaigns recipients` shows the segment a recipient was reached through;
  the List of such a recipient is empty.
- `apikeys` and `subaccounts api-keys` take the `segments:read`,
  `segments:write` and `segments:delete` scopes.
- `templates`, `campaigns` and `campaigns templates` read and write the
  design of the Simple Editor as AhaML (AhaSend Markup). `create` and
  `update` take `--markup-file`, which makes a created design `simple`, and
  `get --markup` writes only the design's AhaML to stdout, so it can be
  edited and written back.
- The views of a template, a template version, a campaign and a campaign
  template show a design's markup, or why the API could not give it as
  AhaML, after the MJML, and a write that carried AhaML shows its warnings.
- A write whose AhaML the API refuses lists each problem under the error,
  with its line and column, outside JSON output.
- A template's variables show their kind: a condition, or a list with the
  fields of its elements.
- The `segments` group manages segments, which find contacts by rules:
  `list`, `get`, `create`, `update`, `delete`, `preview`, which counts the
  contacts a definition matches without saving it, and `members`. A
  segment's rules come from a JSON definition file, whose keys and JSON
  types are checked before the request, with the place of each mistake.
- `contacts segments` lists the segments a contact is a member of.
- `campaigns create` and `campaigns update` take `--segment-id`,
  `--exclude-list-id` and `--exclude-segment-id`, so a campaign sends to
  segments and leaves out the contacts of lists and segments, up to 10
  segments in all. `campaigns update` takes `--clear-lists`,
  `--clear-segments`, `--clear-exclude-lists` and `--clear-exclude-segments`.
  The view of a campaign shows its segments, excluded lists and excluded
  segments after its lists.
- A campaign send or test refused with the `text_unchecked` issue ends with
  a line that says to run the command again, with a new `--idempotency-key`
  for a send that had one.
- The webhook commands take 21 events: `unsubscribed`, `complained` and
  `suppression_deleted` join the others, with eight contact events, such as
  `contact_created` and `contact_added_to_list`, which need an account with
  the marketing features and the `contacts:read` scope, and `lists:read` for
  some. `webhooks create`, `update` and `trigger` take
  `--all-contact-events`, which selects the contact events.
- `webhooks create` and `webhooks update` take `--origins`, which picks whose
  changes the contact and suppression events report, and `webhooks update`
  takes `--all-origins`. The views of a webhook show its origins after its
  events.
- `webhooks list --events` lists only the webhooks that have every event it
  names on.

### Changed

- Plain output prints no blank line under a template's MJML, HTML or text
  that ends in a newline.
- `messages get` and `messages cancel` help describe the message ID as the
  message's `id`, a UUID, which `messages send` prints. The error for an ID
  that is not valid now ends in `not a UUID`.
- `smtp send` help says to find a sent message with `messages list
  --message-id` and the Message-ID the command printed.
- `smtp send`, `messages get` and `messages list --show-details` label the
  Message-ID header `Message-ID`, not `Message ID`.
- The `messages list --message-id` help says it filters by the Message-ID
  header, with its angle brackets.
- The error of `templates create` and `campaigns templates create` without
  a design file or `--editor`, and the "at least one of" errors of the
  `update` commands of `templates`, `campaigns` and `campaigns templates`,
  name `--markup-file`.
- The "at least one of" error of `campaigns update` names the audience
  flags, and its error for an empty `--list-id` names `--clear-lists`.
- The help of `campaigns create`, `update`, `duplicate` and `send` says what
  to do while a segment of the campaign is building, and that of
  `campaigns send` that the API stores its answer to a send with
  `--idempotency-key`, a refusal included. The help of `lists delete` and
  `campaigns delete` says the API refuses while a segment refers to the list
  or campaign.
- `--all-events` of `webhooks create`, `update` and `trigger`, and `all` in
  the events prompt of `webhooks create`, select every event except the
  contact events. The "at least one of" errors of `webhooks update` and
  `webhooks trigger`, and the error for a blank answer to the events prompt,
  name `--all-contact-events`; the error of `webhooks update` also names
  `--origins` and `--all-origins`.

## [0.3.1] - 2026-10-16

This release changes no command. It adds package pages for the install channels.

### Added

- The npm packages have a README, which npmjs.com shows on each package's page.
- The Chocolatey package page shows how to use the CLI, with links to the docs.
- The Docker image has a description, licence and source link, which its GHCR page shows.

## [0.3.0] - 2026-10-16

This release adds the `contacts`, `lists`, `templates` and `campaigns`
commands, and makes exit codes, input checks and output consistent across
commands. Read **Breaking changes** before upgrading scripts written for v0.2.0.

### Breaking changes

#### Exit codes

- An API error exits by its HTTP status in every format: 401 → 2,
  400/422 → 4, 404 → 5, 403 → 6, 429 → 8, others → 1; it exited 1, and 0 with
  `--output json`. See [Exit Codes](USAGE.mdx#exit-codes).
- A network failure or timeout exits with 7 instead of 1, as do a dropped
  listen stream and an unreachable SMTP server.
- A command-line mistake, such as an unknown flag, a bad value or a missing
  argument, exits with 4 instead of 1; `ahasend <group> <unknown>` exited 0.
- An unknown profile, a missing default profile, or a profile with an invalid
  `api_url`, `api_key` or `account_id` exits with 3 before any request (an
  unknown profile exited 5 and a missing default profile 2); `auth status`
  exits nonzero when the key cannot be verified.
- A command with per-item results exits with 1 when any item failed;
  `messages send` counts recipients the API does not queue.
- Ctrl+C or SIGTERM stops `messages send` and `messages cancel` cleanly with
  exit code 130, and a closed stdout pipe exits with 1 instead of SIGPIPE.

#### Input checks

- Input the CLI can check fails with exit code 4 before any request: IDs,
  limits, times, addresses, domains, scopes, lengths, URLs, headers, and files,
  which must be UTF-8 text.
- An empty value fails instead of meaning "not given", so
  `--api-key "$KEY"` with `KEY` unset no longer falls back to the profile, and
  `suppressions wipe --domain ""` no longer wipes every domain
  ([Empty Values](USAGE.mdx#empty-values)).
- Repeatable flags split on commas and trim each item; quote a value that holds
  a comma ([Repeatable Flags](USAGE.mdx#repeatable-flags)).
- Relative times are a whole number and a lowercase unit (`m`, `h`, `d`, `w`,
  `mo`, `y`); `30days`, `7D` and `3h30m` fail
  ([Time Values](USAGE.mdx#time-values)).
- `messages send` refuses `--sandbox-result` without `--sandbox`,
  `--max-concurrency` above 10, a recipients file with no recipients or an
  unknown key, AMP-only content, and inline content with its file flag.
- `messages send --attach` refuses attachments over 29 MB encoded, and an
  HTTP 413 on a send with attachments stops the rest of the send.
- Flags that cannot be combined fail, such as `--api-key` with `--profile`, or
  `domains create --format` with `--output json`.
- **Security:** `messages send` and `smtp send` refuse a `--header` that the
  API or the command sets itself, such as `To`, `Bcc` or `AhaSend-Sandbox`.
- **Security:** `smtp send` refuses a server without STARTTLS, except on
  `localhost`.

#### Prompts

- A missing required value is asked for only on a terminal, and otherwise
  fails; the `routes create` and `webhooks create` wizards ask only for
  required values.
- Destructive commands, `auth logout --all` and `campaigns send` share one
  prompt on stderr: `y` or `yes` goes ahead, anything else exits with 4
  ([Confirmation Prompts](USAGE.mdx#confirmation-prompts)).

#### Flags and defaults

- `--limit` defaults to the API's page size on every list command.
- `domains list --status` and `webhooks list --enabled` filter on the API side,
  and `--enabled=false` lists only disabled webhooks and routes.
- `messages send` leaves tracking to the account's settings unless
  `--track-opens` or `--track-clicks` is given.
- `messages send --idempotency-key` is sent as given for one request, and as
  `<key>-N` for each request of a larger send.
- `webhooks create` defaults to `--scope global`, and `--domains` needs
  `--scope scoped`; `routes create` requires `--recipient` and enables the
  route.
- `domains verify` and `check-dns` take `--show-records` instead of `--verbose`.
- `smtp send --test` needs only `--server`, and reports the connection and the
  login apart.
- `auth login` keeps the existing default profile, `auth logout --profile`
  logs out that profile, and `AHASEND_DEFAULT_PROFILE` is no longer read.
- Deprecated on/off flags, such as `--disable`, take no value and cannot be
  combined with their replacements.

#### Output

- `domains create` writes its DNS instructions and `--format` records, and
  `domains verify` and `check-dns` their tips, to stderr instead of stdout:
  `> zone.txt` saves the domain details, and `2>` the records
  ([`ahasend domains create`](USAGE.mdx#ahasend-domains-create)).
- Errors go to stderr outside `--output json`; prompts, warnings and logs
  always go to stderr, and errors no longer print the usage text.
- `--output json` prints the API's response body unchanged, including for
  delete, `ping` and `suppressions check`, and a CLI error as
  `{"error": true, "message": "..."}` ([JSON Output](USAGE.mdx#json-output)).
- `auth` commands print new JSON shapes, and no format prints the API key.
- `messages send` output counts recipients, and a send of more than 100 prints
  one result per request.
- Table and plain output show local times with their zone and no `N/A`-style
  placeholders; CSV shows RFC 3339 times and empty cells for missing values.
- CSV columns change across most commands ([CSV Format](USAGE.mdx#csv-format)).

### Added

- Install with Homebrew (`brew install ahasend/tap/ahasend`), Scoop, npm
  (`npm install -g @ahasend/cli`), the `install.sh` and `install.ps1`
  scripts, or the .deb and .rpm packages on the release page
  ([installation instructions](https://ahasend.com/docs/cli/installation)).
  WinGet, Chocolatey and AUR packages follow.
- [`contacts`](USAGE.mdx#contact-commands) and
  [`lists`](USAGE.mdx#list-commands) commands manage contacts, contact lists
  and list members.
- [`templates`](USAGE.mdx#template-commands) and
  [`campaigns`](USAGE.mdx#campaign-commands) commands manage transactional
  templates, campaigns and campaign templates, with API key scopes for each.
- `messages send --template-id`, `--from-name` and `--reply-to`.
- `--ip-allow-list` on API keys restricts the IPs that can use them.
- `--idempotency-key` on the `create` commands of most groups.
- `domains create` and `update` take the sending type, DKIM and subdomain
  settings, and domain output shows them.
- `subaccounts unpause-domain`, and the aliases `subaccounts pause`, `resume`
  and `unpause`.
- Suppressions show whether they are protected, and `suppressions list`
  filters by time.
- `messages get` shows delivery attempts and takes a Message-ID;
  `messages list --status` takes `opened` and `clicked`.
- `--schedule` takes relative times such as `2h`.
- Webhook events can be given by key (`delivered`) or type
  (`message.delivered`).
- `routes create` shows the signing secret in every format, and
  `routes list --domain` filters by domain.
- `smtp send --password-stdin`.
- `--non-interactive` on more commands, `-f` for `--force`, and `--before` on
  list commands.
- `stats --sender-domain` is repeatable, and `stats delivery-time` shows
  delivered counts.
- Shell completion of flag values and profile names.
- ARM64 builds for Linux, Windows and Docker, `THIRD_PARTY_NOTICES` in each
  release, and static binaries built with Go 1.27.2.

### Changed

- AhaSend Go SDK 0.3.2; JSON output gains the API's new fields.
- Flags with fixed values take any case.
- DELETE requests are sent once, retries honour `Retry-After`, and
  `messages send --max-retries` is the total number of resends
  ([Rate Limiting](USAGE.mdx#rate-limiting)).
- `messages send` prints where it saved the failed recipients, and saves those
  of unknown outcome, such as after a timeout, to a separate file instead of
  resending them ([`ahasend messages send`](USAGE.mdx#ahasend-messages-send)).
- Table and plain output share one layout per item, name before ID
  ([Output Values](USAGE.mdx#output-values)).
- List output ends with an item count and the page flags; output has no emoji,
  and success messages share one wording.
- A plain-HTTP API, webhook or route URL prints a warning, and so does
  `auth switch` to a profile whose key fails its check.
- Errors say what to check on a network failure and how long a `Retry-After`
  asks to wait.
- Log colors appear only on a terminal and respect `NO_COLOR`; `--debug`
  logs at most 64 KiB of a body.
- `stats` and `messages list` send only the filters you give.
- Hidden prompts handle Backspace, Ctrl+U and a pasted CRLF line ending, and
  restore the terminal when stopped by a signal; `smtp send` hides its password.
- `ahasend --version` shows the build time in RFC 3339, such as
  `2026-10-14T09:00:00Z`. It is the time of the release's commit, so every
  download of a release prints the same version.

### Deprecated

These names still work for one release, hidden from the help, with a notice
on stderr
([Deprecated Flags and Commands](USAGE.mdx#deprecated-flags-and-commands)):

- `--tags` (now `--tag`) and `--cursor` (now `--after`).
- `messages send --text-template`, `--html-template` and `--amp-template`
  (now `--text-file`, `--html-file` and `--amp-file`).
- `--interactive` (prompting is the default; `--non-interactive` turns it off).
- `--enable`, `--disable` and `--disabled` (now `--enabled` and
  `--enabled=false`).
- `webhooks update --no-events` and `--clear-domains` (now `--clear-events`
  and `--scope global`), and the `--no-*` flags of `routes update` (now
  `=false`).
- `domains edit` (now `domains update`).
- `stats --chart`, `--raw`, `--show-totals`, `--classification`, `--trends`
  and `--show-domains`, and `smtp send --credential-id`, which do nothing.

### Removed

- Confirmation prompts no longer fetch the resource or ask you to type a word.
- `webhooks get` and `update` no longer show an always-empty `Secret`.
- `auth status` and `auth login` no longer save the account name or a profile
  `name`, and the configuration file no longer gets a `preferences` block;
  existing keys stay.
- `routes update --clear-recipient`, which the API refused.

### Fixed

- `subaccounts --website` takes a domain, the format the API requires.
- SMTP credential output lists ports 587, 25 and 2525, not 465.
- `auth login`, `status` and `switch` check that the key belongs to the
  account, and `auth logout` picks the next default profile predictably.
- Profile names keep their case and dots, and concurrent `auth` commands no
  longer lose profiles.
- DNS instructions mark records as required or optional, and
  `domains create --format` writes valid BIND, Terraform and Cloudflare records.
- Listen commands forward events in order and unchanged, close cleanly, skip
  unreadable messages, and say when a temporary webhook or route outlives them;
  `routes listen --slim-output` shows the sender, recipient and subject.
- `smtp send` times out on an unresponsive server and shows the server's
  reply on a refused send.
- `smtp send` prints the Message-ID of the message it sent, which
  `messages get` takes, and its JSON output has the keys of its CSV output.
- Large attachments on slow connections no longer time out
  ([Network Limits](USAGE.mdx#network-limits)).
- Files with a UTF-8 byte order mark are read correctly.
- `messages send --recipients` reads `substitution_data` and short CSV rows,
  and keeps large numbers exact.
- `messages send --max-concurrency` above 1 stays within the API's rate limit
  instead of getting HTTP 429s.
- `messages send --show-metrics` prints its metrics, and the progress bar
  fits the terminal.
- Valid addresses such as `o'brien@example.com`, and non-ASCII `--reason`
  text up to the API's limit, are accepted.
- `suppressions check` shows every suppression of the address.
- `routes get` CSV output has its values.
- Commands work with an `account_id` in any accepted UUID form.
- Requests send the CLI's version in `User-Agent`, and release builds report it.
- Help texts list each flag's allowed values and describe what the API does.

### Security

- `~/.ahasend` and its files are readable by your user only.
- `--debug` redacts secrets in request and response bodies.
- Listen commands send the API key in a header, over `wss://` only.
- Listen `--skip-verify` applies only to, and requires, `--forward-to`, which
  refuses URLs with credentials.
- Table and plain output escape control characters from the API.
- Forwarded events are signed with a cryptographically random secret, which
  the listen banner labels `Forwarding secret (signs forwarded events)`.

## Earlier releases

Notes for v0.2.0 and earlier are on the
[GitHub releases page](https://github.com/AhaSend/ahasend-cli/releases).

[Unreleased]: https://github.com/AhaSend/ahasend-cli/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/AhaSend/ahasend-cli/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/AhaSend/ahasend-cli/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/AhaSend/ahasend-cli/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/AhaSend/ahasend-cli/compare/v0.2.0...v0.3.0
