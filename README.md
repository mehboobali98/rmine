# rmine

A command-line client for [Redmine](https://www.redmine.org/) — issues, projects, and time logging, without leaving the terminal. Supports multiple Redmine servers via named profiles.

On the design decisions that only make sense because an agent is the second caller:
[What changed when an agent started using my CLI](https://mehboob.dev/blog/designing-a-cli-for-agents).

## Install

Download a binary from the [Releases](https://github.com/mehboobali98/rmine/releases) page, or build from source:

```sh
go install github.com/mehboobali98/rmine/cmd/rmine@latest
```

`go install` puts the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`) — make sure that's on your `PATH`. `rmine version` reports which build you're running.

### Uninstall

```sh
rmine skill uninstall           # remove the Claude Code skill
rm "$(go env GOPATH)/bin/rmine"
rm -rf ~/.config/rmine          # also drop saved profiles/API keys
```

## Getting started

```sh
rmine config init
```

You'll be asked for your Redmine server URL and a personal API key (found under *My account → API access key* in Redmine). This saves a `default` profile and verifies it works, then offers to install a [Claude Code](https://claude.com/claude-code) skill (`~/.claude/skills/rmine/`) so an AI assistant knows how to drive rmine — run `rmine skill install` any time to (re)install it manually.

```sh
rmine whoami
rmine project list
rmine issue list --project myproject --status open
rmine time log 1234 --hours 1.5 --activity Development --comment "Fixed the login bug"
```

## Common queries

```sh
rmine issue create --project "AssetSonar Scrum Team" --subject "Fix login redirect" --tracker Bug
rmine issue list --assignee me --due-within 2
rmine issue list --assignee me --overdue
rmine issue update 1234 --status "In Progress" --assignee 42
rmine issue list --project "AssetSonar Scrum Team" --status "in progress" --due-next-week
```

`--project`, `--status`, `--tracker` and `--category` match names case-insensitively (`in progress` finds `In Progress`), so you don't need exact server casing. Tracker, status, priority, activity, category and version names also fall back to ignoring whitespace, hyphens and underscores, so `Sub-task` finds `SubTask` and `Internal` finds a category saved as `Internal ` with a trailing space. `--due-within N`, `--due-next-week` and `--overdue` compute the date range for you; `--due-after`/`--due-before` take explicit `YYYY-MM-DD` dates if you need a custom range.

```sh
rmine issue create --project "X" --subject "Ship it" --version "Sprint 42" --attach ./spec.pdf
rmine issue update 1234 --status Resolved --notes "Fixed in build 88"
rmine issue relate 1234 precedes 1235
```

`--sort` takes Redmine's sort syntax — a column, optionally `:asc` or `:desc`, comma-separated for tie-breaks: `--sort due_date`, `--sort "priority:desc,due_date:asc"`.

## Multiple servers

```sh
rmine config add-profile work
rmine config use-profile work        # switch persistently
rmine --profile work issue list      # or override for one command
RMINE_PROFILE=work rmine issue list  # or via env var
rmine config list-profiles
rmine config set-default-project "AssetSonar Scrum Team"
```

A profile's default project supplies `--project` wherever it's omitted: on `issue create`, and as the scope for `issue list` and `time list` — so the project you work in most doesn't need retyping. Pass `--all-projects` on the list commands to search everywhere regardless; combining it with `--project` is an error.

`time list --issue <id>` is not scoped by the default: naming an issue already determines the project, so applying the default on top could only contradict it.

Because that scoping comes from stored configuration rather than from the command you typed, the list commands note it on stderr when the default is what took effect. stdout stays clean, so `-o json` is unaffected.

## Commands

| Command | Description |
|---|---|
| `rmine whoami` | Show the user for the active profile's API key |
| `rmine version` | Show the rmine version |
| `rmine project list` | Browse projects |
| `rmine project view <project>` | Show one project's details, plus its trackers, categories and enabled modules |
| `rmine project categories <project>` | List a project's issue categories |
| `rmine project versions <project>` | List a project's target versions |
| `rmine tracker list` | List the trackers defined on this server |
| `rmine status list` | List the issue statuses, and which of them close an issue |
| `rmine priority list` | List the issue priorities |
| `rmine activity list` | List the time-entry activities |
| `rmine issue list` | List issues (`--project`, `--status`, `--assignee`, `--tracker`, `--version`, `--parent`, `--subject`, `--updated-after`, `--updated-before`, `--due-after`, `--due-before`, `--due-within`, `--due-next-week`, `--overdue`, `--sort`, `--limit`, `--all`, `--all-projects`) |
| `rmine issue view <id>` | Show issue details, its web link, attachments, subtasks and relations (`--comments` to also fetch comments) |
| `rmine issue attachments <id>` | List an issue's attachments (`--download <dir>` to save them all) |
| `rmine issue create` | Create an issue (`--project`, `--subject` required; `--description`, `--tracker`, `--priority`, `--category`, `--assignee`, `--parent`, `--version`, `--start-date`, `--due-date`, `--estimated-hours`, `--done-ratio`, `--field`, `--attach`) |
| `rmine issue update <id>` | Edit an issue (same optional flags as create, plus `--status` and `--notes`) |
| `rmine issue close <id>` | Close an issue (`--status` to pick a specific closed status) |
| `rmine issue comment <id> <note>` | Add a comment (`--attach` to include files) |
| `rmine issue relations <id>` | List an issue's links to other issues |
| `rmine issue relate <id> <type> <other-id>` | Link two issues, e.g. `relate 100 precedes 200` (`--delay` for precedes/follows) |
| `rmine issue unrelate <relation-id>` | Remove a link (prompts unless `-y`/`--force`) |
| `rmine time log [issue-id]` | Log time against an issue, or a project with `--project` (`--hours` required; `--date`, `--activity`, `--comment`) |
| `rmine time list` | List time entries (`--issue`, `--project`, `--user`, `--from`, `--to`, `--sort`, `--limit`, `--all`, `--all-projects`) |
| `rmine time edit <id>` | Edit a time entry |
| `rmine time delete <id>` | Delete a time entry (prompts unless `-y`/`--force`) |
| `rmine config init` / `add-profile` / `use-profile` / `list-profiles` | Manage server profiles |
| `rmine config set-default-project <project>` | Set the active profile's default project (`""` clears it) |
| `rmine skill install` / `uninstall` | Install or remove the rmine Claude Code skill (`--local` for this project, `--force` to overwrite a file rmine didn't write) |

Every command accepts `-o`/`--output json` and `--profile <name>` to target a specific server for that one call.

## Scripting

`issue view` and `issue list -o json` both carry a `url` field with the issue's address in the Redmine web UI, so a script or an assistant reporting a result can hand over a link rather than a bare number.

`-o json` (long form `--output json`) works on every command, including the ones that change something — those print `{"status": "...", ...}` naming what was acted on, so a script or an agent can parse a result from any call. Empty lists come back as `[]`, never `null`. Prompts, warnings and progress notes go to stderr, so they never interleave with the JSON on stdout.

**Failures are JSON too.** Under `-o json` an error prints `{"error": {"message": "...", "status": 422, "errors": [...]}}` on stdout (and the same sentence on stderr for whoever is watching), so every outcome of every call parses:

```sh
rmine issue create --project X --subject "..." -o json \
  | jq -e '.error // empty' && echo "that one failed"
```

`status` and `errors` are present only when Redmine itself rejected the call, which is what separates a 422 worth resending with a fix from a transport failure that should not be resent at all.

For unattended setup, `$RMINE_URL` and `$RMINE_API_KEY` supply the two values `rmine config init` would otherwise prompt for. The key is read from the environment rather than a flag on purpose — a flag would leave it in your shell history and in the process list.

## Field notes

Assignees can be given as a numeric Redmine user ID, the literal `me`, or a person's name. Names are resolved against the project's member list — `/users.json` is admin-only on most instances, while a project's memberships are readable by its members — so a name needs a project in scope: `--project` on `issue list`, and the issue's own project on `issue update`. An exact name wins; failing that a single substring match is accepted, so `--assignee jane` finds Jane Doe. A name matching several members is an error listing them rather than a guess, since assigning work to the wrong person isn't something the caller can detect. If your API key can't read a project's member list, pass a numeric ID.

Custom fields differ per Redmine instance (and sometimes per project/tracker), so they're set generically by numeric ID: `--field 12=staging`, repeatable to set several distinct fields. Passing the same ID more than once (`--field 11=16 --field 11=27`) instead sets that one field to multiple values, for checkbox/multi-select fields. Find a field's ID by inspecting an existing issue that has it set: `rmine issue view <id> -o json`.

`--category` and `--version` are project-specific and matched case-insensitively by name; list a project's valid values with `rmine project categories <project>` and `rmine project versions <project>`. `--tracker`, `--status`, `--priority` and `--activity` are server-wide — `rmine tracker list`, `rmine status list`, `rmine priority list` and `rmine activity list` enumerate them. A name that matches nothing is rejected with the valid spellings listed, so a wrong guess tells you the right answer.

Redmine exposes only a subset of an instance's custom fields on any given tracker, and it does not reject a write that names a field outside that subset — it accepts the write, returns 200, and drops the value. `rmine` compares what it asked for against what the server stored, and reports any field that didn't stick: a warning on stderr, and a `dropped_fields` array under `-o json`. The write itself still succeeds (the issue exists, and the rest of the edit landed), so the exit status stays 0 — resending a `create` would just file a second ticket.

Files are attached with `--attach <path>`, repeatable, on `issue create`, `issue update` and `issue comment`. Each is uploaded before the write it belongs to, so a rejected write leaves nothing half-attached.

Issues are linked with `rmine issue relate <id> <type> <other-id>`, where the type reads left to right — `relate 100 precedes 200` records that #100 precedes #200. Valid types are `relates`, `blocks`, `blocked`, `precedes`, `follows`, `duplicates`, `duplicated`, `copied_to` and `copied_from`; `--delay <days>` applies to `precedes`/`follows` only. `rmine issue relations <id>` lists an issue's links from that issue's point of view (so the other end of a `precedes` correctly reads as `follows`), and `rmine issue unrelate <relation-id>` removes one.

On `issue update`, a flag you don't pass is left alone on the server, and passing an empty one clears the field: `--assignee 0` unassigns, `--parent 0` detaches from the parent, `--category ""` removes the category, `--estimated-hours 0` drops the estimate, and `--description ""` empties the description. `--done-ratio 0` is a real value — 0% — not a clear.

Different Redmine instances (and different projects/trackers within one) can require different mandatory fields — including custom fields. There's no reliable way to know these ahead of time (the field-configuration API is admin-only), so `rmine` just relies on Redmine's own validation: a `create`/`update` that's missing a required field returns the server's exact error (e.g. `redmine returned 422: Category cannot be blank`), naming what's missing so you can retry with it set.

`--subject` (and combining it with other filters) uses Redmine's advanced filter syntax under the hood, which doesn't default to open-only issues the way the plain filters do — expect closed issues in the results too unless you also pass `--status open`.

Shell completion is available via `rmine completion bash|zsh|fish` (see `rmine completion --help` for setup instructions).

## Development

```sh
go build ./...
go vet ./...
go test -race ./...
```

`internal/cli/SKILL.md` is the reference `rmine skill install` writes out; a test checks that it and this README document every flag the CLI registers, so adding a flag means updating both.

Releases are cut with [goreleaser](https://goreleaser.com/) on tag push (see `.github/workflows/release.yml`).
