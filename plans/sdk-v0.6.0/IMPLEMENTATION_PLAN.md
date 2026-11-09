# Go SDK v0.6.0 in the AhaSend CLI

The CLI moves from AhaSend Go SDK v0.3.2 to v0.6.0 and adds commands and
flags for what the SDK added in between: segments, segment audiences for
campaigns, AhaSend Markup (AhaML) for Simple Editor designs, new webhook
events and origins, and the send response's Message-ID header. It also
follows the SDK's breaking changes: the template draft read is gone, a
template read returns the working copy, and a campaign recipient may have no
list.

The work is one branch, `feat/sdk-v0.6.0`, in the worktree
`../cli-worktrees/sdk-v0.6.0`, with at least one commit per stage and one
pull request when all stages are done. Every commit passes the checks listed
in Testing. The last commit deletes this plan. No CLI release is part of this
work.

## What changes

| SDK change | CLI change | Section |
|---|---|---|
| `GetTemplateDraft` and `TemplateDraft` removed; `GetTemplate` returns the working copy; `GetPublishedTemplate` added | `templates get` shows the working copy, `--published` shows the published copy, `templates draft get` is removed | Templates |
| `CampaignRecipient.ListName` is `*string`; `SegmentID`, `SegmentName` added | `campaigns recipients` shows the list or the segment | Campaign audiences |
| `CreateSingleMessageResponse.MessageIDHeader` | `messages send` shows the Message-ID header beside the ID | Messages |
| `DeliveryEvent.ReportMessageID` | `messages get` shows the report message of an out-of-band bounce | Messages |
| `segments:read`, `segments:write`, `segments:delete` scopes | API key commands accept them | API key scopes |
| `TemplateContentInput.Markup`, `TemplateContent.Markup` and `MarkupError`, `MarkupWarnings`, `APIError.Errors` | `--markup-file` input, `--markup` output, markup rows, warnings and problems in the output | AhaML |
| `TemplateVariable.Kind` and `ItemFields` | variables show their kind and list fields | Template variables |
| `SegmentsAPI` | new `segments` group and `contacts segments` | Segments |
| campaign `SegmentIDs`, `ExcludeListIDs`, `ExcludeSegmentIDs` | segment and exclusion flags on `campaigns create` and `update`, rows in `campaigns get` | Campaign audiences |
| `CampaignIssueTextUnchecked` | a line that says to run the command again when a send or test is refused for it | Campaign audiences |
| 11 new webhook event settings, `Origins`, new `GetWebhooksParams` filters | 21 events, `--all-contact-events`, `--origins`, `webhooks list --events` | Webhooks |

## Design

### Templates

`templates get <template-id>` calls `GetTemplate` and shows the working
copy: the draft while the template has one, else the published copy. The
existing Has Draft row says which one it is. With `--published`,
`templates get <template-id>` calls the client method
`GetPublishedTemplate`, added for it, and shows the published copy, which
sends use. `templates list` shows published copies, as the API returns them.

`templates draft get` is removed, with its client method, mock, printer
handler (`HandleTemplateDraft`, `internal/printer/template_draft.go`), tests
and golden sections. `templates draft` holds `discard` alone. A script that
calls `templates draft get` gets the CLI's unknown-command error.

The help of `templates`, `templates get` and `templates draft`, USAGE.mdx
(including the templates group's Scopes paragraph, which names `draft get`)
and README.md describe the working copy and `--published`.

### AhaML

AhaML is the text form of a Simple Editor design
(https://ahasend.com/docs/send-api/ahaml). It applies to templates, campaigns
and campaign templates, which share the content code in
`internal/cmdutil/content.go`.

**Input.** The shared content file flags gain `--markup-file <path>`, so
`templates`, `campaigns` and `campaigns templates` `create` and `update` all
take it. It fills `TemplateContentInput.Markup`. Rules, checked before
authentication:

- `--markup-file` cannot be used with `--mjml-file` or `--html-file`. The API
  refuses MJML with markup and ignores HTML beside it.
- `--text-file` and `--regenerate-text` may be used with it.
- On update, `--markup-file` counts as content for the "at least one of"
  check. Every message that lists the content flags names it, including
  `ReadTemplateEditor`'s "--editor is needed" error.
- An empty `--markup-file` is a validation error, as an empty `--text-file`
  is.
- On `campaigns create`, `--markup-file` is one of the design flags that
  conflict with `--campaign-template-id`.

**Editor.** One helper in `content.go` owns the editor rule for markup,
and all three create commands apply it: when the content has markup, the
editor is `simple` if `--editor` is not given, and `--editor advanced` or
`--editor html` is a validation error. `ReadTemplateEditor`
(`templates create`, `campaigns templates create`) calls it, and so does
`campaigns create`, which reads `--editor` with `cmdutil.ReadOneOf`. For
content without markup, `ReadTemplateEditor` passes `--editor` through when
it is given, sends no editor for MJML or HTML without it, and needs it
otherwise; `campaigns create` with no content and no `--editor` makes an
empty draft.

**Output.** Every view that shows template content (templates, template
versions, campaigns and campaign templates) shows the rows Markup and Markup
Error after the MJML row: Markup holds the markup, and Markup Error holds
`<code>: <message>` when the API could not make the markup. In
`internal/printer/template.go`, the rows come from `templateContentFields`,
which every content view reaches through `templateContentRows`,
`writeTemplateContent`, `templateBodyRows` or `writeTemplateBody`. In CSV,
`markup` and `markup_error` (the text of the two rows) follow `mjml` in
`templateContentColumns`, `templateContentCSVFields` and
`templateContentCSVCells`, and in the header list of every view that writes
those cells, including `templateVersionDetailCSVHeaders`. JSON prints the
API's body.

**`--markup`.** `templates get`, `campaigns get` and
`campaigns templates get` gain `--markup`, which writes only the design's
markup to stdout, adding nothing: to a file or pipe exactly as the API
returned it, and to a terminal (`term.IsTerminal` on stdout, as
`internal/prompt` checks its input) escaped as `EscapeDisplayText` escapes
table and plain output. So
`ahasend templates get <id> --markup > design.ahaml` saves it for editing,
and `ahasend templates update <id> --markup-file design.ahaml` writes it
back. It works with `--published`. Rules, where `<what>` is `template`,
`campaign` or `campaign template`:

- `--markup` with `--output` is a validation error, checked before
  authentication.
- With no content: `<what> <id> has no content`.
- With a `markup_error`:
  `the design could not be read as AhaML: <code>: <message>`.
- With content but neither markup nor a markup error:
  `<what> <id> has no AhaML: only a Simple Editor design has one`.
- Each of these three content failures is a plain error, which exits 1.

**Problem form.** A markup problem or warning prints on one line; each of
its fields is escaped with `EscapeControl`, so a newline in a message cannot
split it:

```
line <line>, column <column>: <message> (<code>) at <path>; hint: <hint>
```

`line <line>, column <column>: ` is left out when the problem has no line,
` at <path>` when it has no path, and `; hint: <hint>` when it has no hint.

**Warnings.** A write that carried markup returns `markup_warnings`. In table
and plain output, the single-item view of a template, campaign or campaign
template (the view of `HandleSingleTemplate`, `HandleSingleCampaign` or
`HandleSingleCampaignTemplate`, which `get`, `create`, `update` and the other
commands that return one of them use) with at least one warning shows the
row Markup Warnings after the content rows, one warning per line in the
problem form. The CSV header of those three views gains `markup_warnings`.
Its cell holds the warnings as compact JSON when there is at least one, and
is empty otherwise, also when the SDK returns an empty list. Each of those
views gets a field list of its own, so `markup_warnings` reaches none of the
views that share their fields: `templates list` (`templateListFields`),
`campaigns templates list` (`campaignTemplateListFields`), and
`campaigns send`, whose `HandleCampaignSend` prints its campaign with the
campaign detail fields (`campaignDetailFields`, written in CSV by
`writeCampaignCSV`).

**Problems in errors.** The API refuses AhaML it cannot read with up to 100
problems, which the SDK puts in `APIError.Errors`. In table, plain and CSV
mode, `printer.HandleError` prints the `Error:` line with its hint as it is,
then each problem on its own line, indented by two spaces, in the problem
form. JSON mode prints the API's body unchanged. Every command whose error
reaches `HandleError` gets this; the per-call errors that
`HandleAPIResults` prints, for multi-request sends and cancels, are not
markup writes.

### Template variables

`formatTemplateVariables` adds each variable's kind and, for a list, its
item fields. The Variables row joins the variables with ", ", so each one
carries no comma:

```
first_name (required)
vip (condition; optional)
items (list of name/price; required)
tags (list; optional)
```

A `value` variable, and one with an empty kind, prints without its kind. A
list with no item fields prints `list`. A kind the CLI does not know prints
as the API names it, in the place of `condition`.

### Segments

A new group, `segments`, registered in `cmd/root.go` between `routes` and
`smtp`, built like the `lists` group.

| Command | SDK call | Output |
|---|---|---|
| `segments list` | `GetSegments` | segment list |
| `segments get <segment-id>` | `GetSegment` | segment detail |
| `segments create --name <name> --definition-file <path> [--description <text>] [--timezone <tz>]` | `CreateSegment` | segment |
| `segments update <segment-id> [--name] [--description] [--definition-file] [--timezone]` | `UpdateSegment` | segment |
| `segments delete <segment-id> [--force]` | `DeleteSegment` | success line |
| `segments preview --definition-file <path> [--timezone <tz>]` | `PreviewSegment` | preview |
| `segments members <segment-id>` | `GetSegmentContacts` | member list |
| `contacts segments <id-or-email>` | `GetContactSegments` | segment list |

- `list`, `members` and `contacts segments` take the pagination flags that
  `lists list` takes.
- `create` takes the idempotency key flag, as other create commands do.
  `update` and `delete` follow `lists update` and `lists delete`, including
  `delete`'s confirmation unless `--force` is set.
- `update` needs at least one flag. `--description ""` clears the
  description, and `--timezone ""` sets UTC. Timezone left out on create
  means UTC.
- The help of `create` says that a new segment is `building` until its
  members are found, that a campaign to it cannot be sent until it is
  `ready`, and that `segments get` shows its state. The help of `update` says
  that a change of the definition or the timezone builds the members again,
  that any update of a `failed` segment builds it again, and that a change of
  only the name or description of any other segment leaves its members and
  its state as they are; a failed segment shows its failure reason.
- `--name` and `--description` get the local checks that `lists create`
  gives them, with the lengths the API spec gives for segments, and the
  segment commands join the name and description catalogs in
  `cmd/validation_before_auth_test.go`.
- `preview` saves nothing, and prints no success line before its view.

**Views.**

- *Segment*: the fields `name`, `id`, `description`, `state`,
  `failure_reason`, `member_count`, `timezone`, `last_evaluated_at`,
  `created_at`, `updated_at` and `definition`, with the headers Name, ID,
  Description, State, Failure Reason, Members, Timezone, Last Evaluated,
  Created, Updated and Definition. An unset failure reason or last
  evaluation is empty, and plain output leaves it out, as it leaves out every
  empty value. The definition is indented JSON in table and plain output and
  compact JSON in CSV.
- *Segment detail* (`get`): the segment fields with `marketable_count`
  (Marketable) after `member_count`. A null marketable count is `unknown` in
  table and plain output and an empty cell in CSV. The help of `get` says
  that Members counts every contact that matches and Marketable counts those
  a campaign would reach.
- *Segment list*: the table shows Name, ID, State, Members and Updated; plain
  and CSV show every segment field.
- *Member list*: `email`, `contact_id`, `entered_at` (Email, Contact ID,
  Entered).
- *Preview*: the line `Count: <n>`, or `Count: not counted in time` when the
  API returns no count, then the sample contacts, up to 10: a table with the
  columns Email and Contact ID in table output, and in plain output each
  contact's `Email:` and `Contact ID:` lines, as the plain member list
  prints them. CSV has the header `count`, `email`, `contact_id` and one row
  per sample contact, with the count in each row and an empty count cell
  when the API returns none; with no sample contacts it has one row with the
  count alone, and table and plain output show the Count line alone.

**Definition file.** `--definition-file` names a JSON file that holds one
segment definition object as the API documents it:

```json
{
  "schema_version": 1,
  "match": "all",
  "conditions": [
    {"field": "attributes.plan", "op": "equal", "value": "pro"},
    {"match": "any", "conditions": [
      {"field": "attributes.country", "op": "equal", "value": "DE"},
      {"field": "campaign_count", "op": "opened_at_least",
       "value": {"count": 3, "since": "90 days ago"}}
    ]}
  ]
}
```

The Long help of `create`, `update` and `preview` shows this example,
brought in by a sentence rather than an `Example:` heading, which
`assertNoExamplesInLong` refuses.

The CLI reads the file with `cmdutil.ReadInputFile` and checks its shape
before it decodes it:

- The file is a JSON object, the definition. It may hold `schema_version` (a
  JSON integer), `match` (a string) and `conditions` (an array). A JSON
  integer here is a number written with no fraction or exponent that fits a
  Go `int`.
- An entry of the definition's `conditions` is an object. One with a `match`
  key is a group and may hold `match` (a string) and `conditions` (an
  array); any other entry is a condition.
- Every entry of a group's `conditions` is a condition object. One that has a
  `match` key fails as a group inside a group, before its other keys are
  checked.
- A condition may hold `field` and `op` (strings) and `value`: a string, a
  number, `true`, `false`, `null`, or a count object.
- A count may hold `count` (a JSON integer) and `since` (a string).
- A number in `value`, in any notation, is one that a float64 keeps: it is
  finite in a float64, and if its value has no fraction, as `1e15` and `3.0`
  have none, it lies between -2^53 and 2^53, since the SDK sends numbers as
  float64, which holds every such value in that range and not all beyond it.
  Other numbers, such as `0.1`, pass.
- No object names a key twice.

The keys of one object are checked in sorted order. Each failure is a
validation error, checked before authentication, that names the path, for
example:

```
--definition-file: conditions[1].conditions[0].vlaue is not a key of a segment condition
--definition-file: conditions[0].value must be a string, number, true, false, null or a count
--definition-file: conditions[1].conditions[0] is a group inside a group, which the API does not allow
```

An empty file, or one that is not JSON, is a validation error that names
`--definition-file` and the JSON decoder's reason. The CLI then decodes the
file into `common.SegmentDefinition`; a failure there, which the shape check
leaves no known way to reach, is a validation error that names
`--definition-file` and the decoder's message. The CLI checks nothing more;
the API checks fields, operators and values and names the path of each
problem.

**Printer.** New files in `internal/printer`: `segment.go` (segment, segment
detail, segment list), `segment_preview.go` and `segment_member.go`, with
handlers `HandleSegmentList`, `HandleSingleSegment`, `HandleSegmentDetail`,
`HandleSegmentPreview` and `HandleSegmentMemberList`, built like
`internal/printer/list.go`.

### Campaign audiences

**Flags.** `campaigns create` and `campaigns update` take four repeatable ID
flags: `--list-id`, `--segment-id`, `--exclude-list-id` and
`--exclude-segment-id`. One reader in `cmd/groups/campaigns/helpers.go`,
`readAudience`, parses all four: it parses each ID as a UUID and counts the
distinct IDs. Limits, checked before authentication: at most 50
distinct lists across `--list-id` and `--exclude-list-id`, and at most 10
distinct segments across `--segment-id` and `--exclude-segment-id`. The
errors, with the real count:

```
--list-id and --exclude-list-id take at most 50 lists in all, got 51
--segment-id and --exclude-segment-id take at most 10 segments in all, got 11
```

The CLI checks only the IDs a command gives; the API checks the stored
sets.

On `update`, each flag replaces its set, and four flags empty a set:
`--clear-lists`, `--clear-segments`, `--clear-exclude-lists` and
`--clear-exclude-segments`. Each one cannot be used with the ID flag of its
set. An emptied set is sent as `[]`, which `UpdateCampaignRequest` sends for
a non-nil empty slice; a set no flag names is left out and keeps its value.
The "at least one of" message of `update` names the three ID flags beside
`--list-id` and the four clear flags. The API refuses a campaign left with no
list and no segment.

**Output.** The campaign detail view adds the rows Segments, Excluded Lists
and Excluded Segments after Lists, in the format of the Lists row. CSV adds
`segment_ids`, `exclude_list_ids` and `exclude_segment_ids` after `list_ids`.
`campaigns send` prints its campaign with the campaign detail fields, so
its output gains the same rows and columns.

**Recipients.** `campaigns recipients` shows a recipient reached through a
segment. `campaignRecipientFields`, which plain and CSV output use, gains
`segment_id` (Segment ID) and `segment_name` (Segment) after `list_name`, and
the table gains the Segment column after List. The List value is empty when
`list_name` is null, and the Segment values are empty when the recipient
came through a list.

**Building segments.** While a segment of the campaign is building,
`campaigns create`, `duplicate`, and an `update` that changes the audience
can fail with 409 after the API waits for the segment; the same command, with
the same idempotency key or a new one, can run again, because the server's
`errCampaignAudienceBusy` releases the key. The help says so, as the server
behaves, whatever the SDK's doc comments on `CreateCampaign`,
`UpdateCampaign` and `DuplicateCampaign` say. `campaigns send` fails
with 400 and the API's message to schedule the campaign when the segment is
ready. The help of these four commands says so and points to `segments get`
for the segment's state. The help of `campaigns send` also says that the API
stores its answer to a send with `--idempotency-key`, a refusal included, and
gives it again to a run with the same key for 24 hours, so a refused send
runs again with a new key or without the flag. The help of `lists delete` and
`campaigns delete` says the API refuses the delete while a segment refers to
the list or campaign.

**text_unchecked.** When the API refuses `campaigns send` or `campaigns test`
because of campaign issues and one of them is `text_unchecked`,
`campaignNotReadyError` adds a line after the issues, and the command exits
4. For `campaigns test`, and for `campaigns send` without
`--idempotency-key`, the line is:

```
text_unchecked clears on its own: run the command again
```

For `campaigns send` with `--idempotency-key`, whose refusal the API stores
(Building segments, above), it is:

```
text_unchecked clears on its own: run the command again with a new --idempotency-key
```

### Webhooks

**Events.** `internal/webhooks/events.go` `Events` lists all 21 events the
SDK and the server know. The 11 that the CLI adds, by Name and Type:

| Name | Type |
|---|---|
| `unsubscribed` | `message.unsubscribed` |
| `complained` | `message.complained` |
| `suppression_deleted` | `suppression.deleted` |
| `contact_created` | `contact.created` |
| `contact_updated` | `contact.updated` |
| `contact_deleted` | `contact.deleted` |
| `contact_added_to_list` | `contact.added_to_list` |
| `contact_removed_from_list` | `contact.removed_from_list` |
| `contact_subscribed` | `contact.subscribed` |
| `contact_unsubscribed` | `contact.unsubscribed` |
| `contact_complained` | `contact.complained` |

The message events go after `clicked`, `suppression_deleted` after
`suppression_created`, and the contact events after `dns_error`. `Event`
gains a `Contact bool`, true for the eight contact events. The list drives
parsing, completion, the create prompt, the listen filter, trigger and the
display order.

Four places map an event to an SDK field: `setCreateEvents`
(`cmd/groups/webhooks/create.go`), `setUpdateEvents` (`update.go`),
`formatWebhookEvents` (`internal/printer/utilities.go`) and the list
filter's mapping function in `internal/client/client.go` (List filter,
below). Each place has a test, in its own package, over every event of
`Events`: each setter, given one event alone, sets exactly one field, and no
two events set the same field; the list filter's test reads that field from
the request's query; `formatWebhookEvents`, given one `On*` field alone set
true, prints that event's name alone. An event added to `Events` and missed
in a place fails that test.

**Selecting events.** `webhooks create`, `webhooks update` and
`webhooks trigger` take:

- `--events <name or type>` (repeatable): any of the 21;
- `--all-events`: every event except the contact events, the 13 that any
  account can use;
- `--all-contact-events`: the 8 contact events.

`--all-events` and `--all-contact-events` may be used together; neither may
be used with `--events`. The "at least one of" messages of `webhooks update`
and `webhooks trigger` name every flag that can satisfy them, including
`--all-contact-events`, `--origins` and `--all-origins` where the command
takes them. On `update`, the event flags name the webhook's whole set of
events, so every event they do not name is turned off, and `--clear-events`
turns off all 21; `--clear-events` cannot be used with any of the three
event flags. The help of each command says that contact events need an
account with the marketing features and an API key with `contacts:read`, and
`lists:read` as well for the list, subscribe, unsubscribe and complaint
contact events; the API refuses them otherwise.

`webhooks create` asks for events when no event flag is given. The prompt
lists the 21 names and takes `all` as `--all-events`; its error for a blank
answer names `--events`, `--all-events` and `--all-contact-events`. The
prompt's text and the flag description of `--all-events` both say "every
event except the contact events".

**Origins.** `webhooks create` and `webhooks update` take `--origins`
(repeatable; `recipient`, `api`, `dashboard`, `system`, from
`common.WebhookOrigin*`, with completion). Origins pick whose changes the
contact and suppression events report. On create, no `--origins` leaves
`origins` out of the request, which the API takes as every origin. On
update, `--origins` replaces the set, and `--all-origins` sends all four,
because the API keeps the stored set when it gets none; `--all-origins`
cannot be used with `--origins`. Any other value of `--origins` is a
validation error, checked before authentication. The field `origins`
(Origins) follows `events` in `webhookListFields`, `webhookDetailFields` and
`createdWebhookFields`, but not in the table columns. It shows the values;
when the API returns null it shows `all` in table and plain output and an
empty cell in CSV, as the Domains field does for a webhook of every
domain.

**List filter.** `webhooks list` gains `--events <name or type>` (repeatable),
which lists only webhooks that have every named event on. It sets the
matching `On*` field of `api.GetWebhooksParams` to true.
`client.ListWebhooks` takes the names of the filter's events as a
`[]string` beside the `enabled` filter, and a function in
`internal/client/client.go` maps each name to its `On*` field. An unknown
name is an error; the command passes only names that `ParseEvents`
accepted.

**Listen.** The `--events` filter of `webhooks listen` accepts the 21
events through `ParseEvents`. Every event is shown and forwarded alike, and
slim output is the event type alone.

### Messages

**Send.** For a single-request send, `HandleCreateMessage` adds the field
`message_id_header`, labelled `Message-ID` as in `messages get`, after `id`
in table, plain and CSV output. Its value is empty when the API returns
none. The help of `messages send` says that `id` is what `messages get` and
`messages cancel` take, and that the Message-ID header is what
`messages list --message-id` takes. Multi-request sends keep their per-batch
summaries.

**Report message.** `DeliveryEvent.ReportMessageID` is set only on an
out-of-band bounce, and names the bounce report message. In `messages get`,
table output adds a Report Message ID column to the delivery attempts table
when at least one attempt has one; plain output prints
`Report Message ID: <id>` under each attempt that has one. CSV writes the
attempts as JSON, which includes it.

**Retries.** A send or a campaign test whose template check cannot run gets
503 with a Retry-After. `messages send` retries it after the Retry-After with
the same idempotency key, as it does any 5xx answer;
`TestBatchProcessor_WaitsTheRetryAfterOfA503` (`internal/batch/batch_test.go`)
pins this. `campaigns test` sends once, with SDK retries off, so its 503
prints as an API error.

### API key scopes

`validation.ScopeGroups` (`internal/validation/scopes.go`) gains
`{"Segments", {"segments:read", "segments:write", "segments:delete"}}` after
Lists, in the API's order. The scope tests and the scope list in USAGE.mdx
follow it in Stage 1. README.md's segments section, written in Stage 3,
names the segment scopes, as its other group sections name theirs.

### Docs and changelog

Each stage updates, for what it changes:

- the command help (Long and Examples);
- USAGE.mdx: a `#### \`ahasend ...\`` entry with its `**Flags:**` list for
  each command, and one `**Scopes:**` paragraph per group, as the contacts,
  lists, templates and campaigns groups have. Which segments command needs
  which scope comes from the SDK's `openapi/openapi.yaml` at v0.6.0;
- README.md: the Core Commands table and the group sections;
- CHANGELOG.md under `## [Unreleased]`, under the Keep a Changelog heading
  that fits (`### Added`, `### Changed`, `### Removed`, `### Fixed`), with a
  change that can break a script under `### Breaking changes` instead, as the
  0.3.0 release does. These go under `### Breaking changes`: the removal of
  `templates draft get`; the working copy in `templates get`; every CSV
  header that gains a column before its last one (`messages send`,
  `campaigns recipients`, the template, template version, campaign and
  campaign template views, and the webhook views); the event flags of
  `webhooks update`, which name the whole set of 21 events, so an added event
  they do not name, such as a contact event turned on in the dashboard, is
  turned off; and the text of the list limit error.

## Testing

- Unit tests use `mocks.MockClient` through `testutil.InjectClient` and
  `RunCommand`, or `testutil.RunWithClient`, as the existing groups do.
  Every client method this work adds has a test against an `httptest`
  server that checks the method, path, query and body.
- Printer output is pinned by the golden files in
  `internal/printer/testdata`. Every handler this work adds or changes gets
  full and sparse renders in `renders_test.go`; every list handler it adds
  also joins `nilLists` in `golden_test.go` and `everyList` in
  `output_style_test.go`, which render no response and pages of 0, 1 and 2
  items. The golden files are written with
  `go test ./internal/printer -run TestOutputMatchesTheGoldenFiles -update`,
  and the diff is read line by line before the commit.
- The commands and flags this work adds or removes are reflected in the
  catalog tests in `cmd/`: `command_tree_test.go`, `root_contacts_test.go`,
  `csv_columns_test.go`, `json_output_test.go`,
  `validation_before_auth_test.go`, `completion_test.go`,
  `fixed_values_test.go`, `success_lines_test.go`, `idempotency_test.go` and
  `pagination_test.go`; in the mock check in `internal/mocks/client_test.go`;
  in the printer's handler catalogs `TestJSONHandlersPrintRecordedAPIBody`
  (`internal/printer/json_raw_test.go`) and
  `TestSingleViewsShowTheirFieldsInOneOrder` and
  `TestSummaryViewsShowTheirFieldsInOneOrder`
  (`internal/printer/view_order_test.go`); and, for a delete, in
  `TestDeleteRequestsAreSentOnce` (`internal/client/client_test.go`).
- `examples_test.go` checks that the commands and flags in every Example
  exist, and `TestCommandGroupsFollowHelpConventions` keeps Long help lines
  under 80 columns.
- Every test package runs under `internal/testenv` (`testenv.Main` in its
  `TestMain`, `testenv.Run` in `test/integration`), which gives it a
  temporary HOME and refuses network traffic that is not loopback.
- A built binary runs only inside
  `unshare -rn sh -c 'ip link set lo up && …'`, with a temporary HOME whose
  profile points at a stub server on 127.0.0.1. No command runs with
  `--api-key` or `--account-id`, and `auth login` never runs without
  `--api-url`: these reach the production API.
- Before each commit, the checks CI runs: `go build ./...`, `go vet ./...`,
  `make test`, `make lint`, `make deps-check` and `make notices-check`.

## Stage 1: SDK v0.6.0 and its breaking changes

**Goal**: The CLI builds on SDK v0.6.0 and follows its breaking and changed
answers.

**Work**:
- `github.com/AhaSend/ahasend-go v0.6.0` in go.mod, then `go mod tidy` and
  `make notices`, so go.sum and THIRD_PARTY_NOTICES name v0.6.0 only.
- Templates: `templates get` working copy, `--published`, removal of
  `templates draft get` and everything that only it used.
- Campaign recipients: nullable list name and the Segment column.
- Messages: `message_id_header` in send output, report message ID in
  `messages get`.
- API key scopes: the Segments group.
- Retries: nothing to build; `TestBatchProcessor_WaitsTheRetryAfterOfA503`
  passes on v0.6.0.
- Docs and CHANGELOG for all of the above, with the breaking changes.

**Success Criteria**: The checks in Testing pass. No code refers to
`responses.TemplateDraft` or `GetTemplateDraft`.

**Tests**:
- `templates get` calls `GetTemplate`; `templates get --published` calls
  `GetPublishedTemplate`; the client test checks the path
  `/v2/accounts/<id>/templates/<id>/published`.
- `templates draft get` is an unknown command; `templates draft discard`
  works.
- A recipient with a null list name and a segment prints the segment name,
  an empty List value in table and CSV output, and no List line in plain
  output.
- A send answer with `message_id_header` prints it labelled Message-ID; one
  without it prints an empty Message-ID value in table and CSV output and no
  Message-ID line in plain output.
- A delivery attempt with a report message ID shows the column in table
  output and the `Report Message ID:` line in plain output; a message with
  none shows neither.
- `apikeys create --scope segments:read` passes validation.

**Status**: Complete

## Stage 2: AhaML and markup problems

**Goal**: Users read and write Simple Editor designs as AhaML, and see markup
warnings and problems in every output format.

**Work**:
- `--markup-file` on the six create and update commands, with its rules and
  the editor helper.
- Markup and Markup Error rows and CSV cells in every content view;
  `--markup` on the three get commands.
- Markup warnings in the template, campaign and campaign template views.
- `APIError.Errors` in `HandleError`.
- Template variable kinds.
- Docs and CHANGELOG.

**Success Criteria**: A template can be read with `--markup`, edited and
written back with `--markup-file` against a stub server. A refused AhaML
write prints each problem with its line and column outside JSON mode.

**Tests**:
- `--markup-file` with `--mjml-file` or `--html-file` is a validation error
  before authentication; with `--editor html` or `--editor advanced` on
  each create command too; with `--campaign-template-id` on
  `campaigns create` too; an empty `--markup-file` too. With `--text-file`
  or `--regenerate-text` it is accepted.
- `templates create --markup-file f`, `campaigns create --markup-file f` and
  `campaigns templates create --markup-file f` with no `--editor` send
  `editor: simple` and `content.markup` with the file's text;
  `campaigns create --name Draft` with no content and no `--editor` sends no
  editor.
- `templates update`, `campaigns update` and `campaigns templates update`
  with `--markup-file f` alone pass the "at least one of" check and send
  `content.markup`; with no flag, their error names `--markup-file`.
- `templates get --markup`, `templates get --published --markup`,
  `campaigns get --markup` and `campaigns templates get --markup` write the
  stub's markup byte for byte. With a `markup_error`, `--markup` fails with
  the code and message (exit 1); for an MJML design it fails with the
  no-AhaML error; with no content it fails with the no-content error; with
  `--output json` or `--output table` it is a validation error (exit 4).
  Markup with a control character prints escaped to a terminal and byte for
  byte to a pipe.
- The golden renders of a template, a template version, a campaign and a
  campaign template show the Markup row for content with markup and the
  Markup Error row for content with a markup error, and their CSV headers
  and cells line up.
- An API error with three problems (one with line, column and path, one with
  no line, one with a hint) prints them in the problem form in table, plain
  and CSV mode, and the API's body unchanged in JSON mode. A problem and a
  warning whose message holds a newline and a control character each print
  on one line, escaped.
- A template, campaign and campaign template write answer with two warnings
  shows the Markup Warnings row and the `markup_warnings` CSV cell; one with
  an empty list shows no row and an empty cell. The CSV headers of
  `templates list`, `campaigns templates list` and `campaigns send` do not
  hold `markup_warnings`.
- `templates create` with neither content nor `--editor` fails with an
  error that names `--markup-file` among the content flags.
- Variables print in each form in Template variables: value, empty kind,
  condition, list with fields, list without fields, and an unknown kind.

**Status**: Complete

## Stage 3: Segments and segment audiences

**Goal**: Users manage segments, see a contact's segments, and send campaigns
to segments with list and segment exclusions.

**Work**:
- The `segments` group, the client methods, mocks and printer handlers.
- The definition file reader with its shape check.
- `contacts segments`.
- Campaign audience flags, clear flags, limits, output rows and CSV columns.
- Help for building segments; the `text_unchecked` line.
- Docs and CHANGELOG.

**Success Criteria**: Every segment command works against a stub server in
every output format. A `campaigns create` with `--segment-id` and
`--exclude-list-id` sends the right request.

**Tests**:
- Each segment command, and `contacts segments`, calls its SDK method with
  the right arguments and prints the answer in table, plain, CSV and JSON
  (JSON is the API's body).
- The definition reader accepts the example in Segments and a `value` of
  `0.1`. It names the path of an unknown key in a condition, a group, a count
  and the definition; of an entry that is not an object; of a string
  `schema_version`, a `schema_version` of `1.0`, a `conditions` that is not
  an array, an array `value`, a string `count`, a `count` of `1e2`, a
  `value` of 2^53 + 1, `9007199254740993.0` or `1e400`, and a key named
  twice. It fails on a group inside a group with the group-inside-a-group
  error, and refuses an empty file, a file that is not JSON and a file that
  is not a JSON object.
- `segments update` with no flag is a validation error; `--description ""`
  sends an empty description; `--timezone ""` sends an empty timezone.
- `segments preview` with a null count prints `Count: not counted in time`
  and the sample contacts; its CSV has one row per sample contact, and one
  row when there are none.
- `segments get` with no marketable count prints `unknown`.
- `segments delete` asks for confirmation, sends nothing when the answer is
  no, and asks nothing with `--force`.
- `campaigns create` with 41 lists and 10 exclude lists is a validation
  error; with 10 segments and 1 exclude segment too.
- `campaigns update --clear-exclude-segments` sends
  `"exclude_segment_ids": []` and leaves out the other sets;
  `--clear-lists --list-id x` is a validation error; `campaigns update` with
  no flag names the audience flags in its error.
- A campaign with segments and exclusions shows the Segments, Excluded
  Lists and Excluded Segments rows and CSV columns in `campaigns get` and
  `campaigns send` output.
- A send refused with issues `text_unchecked` and `lists_missing` prints both
  issues and the `text_unchecked` line, and exits 4; with `--idempotency-key`
  it prints the line that asks for a new key; a refused test prints the
  plain line.

**Status**: Complete

## Stage 4: Webhook events, origins and filters

**Goal**: Webhooks can use all 21 events and origins, and the list can be
filtered by event.

**Work**:
- The 21 events, `Contact`, the four mapping places and their tests.
- `--all-contact-events` on create, update and trigger; `--all-events` and
  `all` in the create prompt as the 13 non-contact events; event flags on
  `update` naming the whole set; the "at least one of" messages.
- `--origins` and `--all-origins`; the Origins field.
- `webhooks list --events` and the `ListWebhooks` change.
- Docs (including the events table in USAGE.mdx) and CHANGELOG.

**Success Criteria**: `webhooks create --all-events --all-contact-events`
sends all 21 settings as true; `webhooks list --events contact_created`
sends `on_contact_created=true`.

**Tests**:
- Each mapping place's test fails when an event is left out of it.
- `--all-events` sends the 13 non-contact settings true and the contact ones
  false; `--all-contact-events` the reverse; `--events` with either is a
  validation error. In `cmd/groups/webhooks/events_test.go`, the test of
  `--all-events` on create and update, named
  `TestCreateAndUpdateWithAllEventsSetEveryNonContactEvent`, and
  `TestTriggerSendsTheEventTypesTheEndpointTakes` check the 13 events, and
  `TestAllEventsHasOneDescription` checks the `--all-events` description.
- `webhooks update` and `webhooks trigger` with no flag name every flag that
  can satisfy them in their errors; `--clear-events --all-contact-events` is
  a validation error.
- The create prompt lists the 21 names and says that `all` means every event
  except the contact events.
- The create prompt answered `all` sends the 13 non-contact settings true;
  a blank answer's error names the three event flags;
  `--all-contact-events` alone makes no prompt.
- `webhooks update --events delivered` sends all 21 settings, only
  `on_delivered` true; `--clear-events` sends all 21 false.
- `webhooks create --origins api --origins system` sends both origins, and
  `webhooks create` without `--origins` sends none.
- `webhooks update --all-origins` sends all four origins;
  `--origins api --all-origins` is a validation error; `--origins nobody` is
  a validation error before authentication.
- A webhook with null origins shows `all` in table and plain output and an
  empty cell in CSV, and one with two shows both, in the detail and created
  views and in the plain and CSV list views.
- `webhooks trigger --all-contact-events` sends the 8 contact types.
- `webhooks listen --events contact.created` keeps a contact event and drops
  a message event.

**Status**: Complete

## Decision log

- **`templates get` shows the working copy, `--published` the published
  copy, and `templates draft get` is removed.** This follows the API, where
  read, change and write back builds on what was read. Rejected: keeping
  `templates get` on the published copy and pointing `templates draft get`
  at the working copy, which keeps old scripts working but makes get, edit,
  update change the published copy rather than the draft. Rejected: a hidden
  `draft get` that points to `templates get`, which keeps a command for a
  read the API does not have.
- **One pull request for all stages.** One review covers the whole
  upgrade. Rejected: one pull request per stage, merged in order.
- **`--all-events` selects every event except the contact events, and
  `--all-contact-events` the contact events.** Rejected: putting
  contact events in `--all-events`, which makes the API refuse every
  existing `--all-events` call from an account without the marketing
  features. Rejected: `webhooks trigger --all-events` sending no event list
  so the server picks every allowed event, which gives the flag a different
  meaning in trigger than in create and update.
- **A segment definition is a JSON file in the API's shape.** It matches the
  API docs, and the API names the path of each problem. Rejected: flags that
  build conditions, which would need a flag syntax for about 40 operators,
  nested groups and count values.
- **The CLI checks the shape of a definition file: its keys and JSON
  types.** The SDK's decoder drops unknown keys inside conditions, decodes a
  group inside a group as an empty condition, and reports a type error by Go
  field rather than by path, so a typo would change the rule without an error
  or fail with a message that names no place in the file. Rejected: checking
  fields, operators and values, which copies rules the API owns and
  changes.
- **`--markup` writes the markup alone to stdout.** It gives a read, edit,
  write loop with no other tool. Rejected: a `--markup-file` on get commands,
  which gives one flag name a read meaning on update and a write meaning on
  get. Rejected: only documenting `--output json` with `jq`.
- **Template variables carry no commas.** Rejected:
  `items (list of name, price; required)`, which a reader cannot split from
  the next variable in the Variables row.
- **Markup warnings print one per line, and as JSON in CSV.** The problem
  form holds a comma, and the CSV view writes a message's delivery attempts,
  a list of structured items, as JSON too. Rejected: joining the warnings
  with ", " in one cell.
- **`campaigns test` sends once.** The API answers its daily limit with
  429, which a retry cannot pass. Rejected: retrying the template check's
  503 only, which needs a retry rule of its own for one command.
- **Origins are checked against the four known values.** The CLI checks its
  other fixed values the same way and fails before authentication. Rejected:
  passing any string to the API.
- **Webhook events map to SDK fields in four places, each with a test.**
  Rejected: a table of setter functions on `Event`, which rewrites working
  code for no change in behaviour.
- **Each campaign audience set has its own clear flag.** It follows
  `--clear-from` and `--clear-reply-to`. Rejected: an empty `--list-id ""`
  meaning "clear".
- **`segments members` is one command.** Segment members are read only, so a
  group like `lists members` would hold one command.
- **Webhook listen shows every event alike.** Rejected: per-event details
  in slim output, which webhook listen has for no event.
