# rmine

A command-line client for [Redmine](https://www.redmine.org/) — issues, projects, and time logging, without leaving the terminal. Supports multiple Redmine servers via named profiles.

## Install

Download a binary from the [Releases](https://github.com/mehboobali98/rmine/releases) page, or build from source:

```sh
go install github.com/mehboobali98/rmine/cmd/rmine@latest
```

`go install` puts the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`) — make sure that's on your `PATH`.

### Uninstall

```sh
rm "$(go env GOPATH)/bin/rmine"
rm -rf ~/.config/rmine   # also drop saved profiles/API keys
```

## Getting started

```sh
rmine config init
```

You'll be asked for your Redmine server URL and a personal API key (found under *My account → API access key* in Redmine). This saves a `default` profile and verifies it works.

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
rmine issue update 1234 --status "In Progress" --assignee 42
rmine issue list --project "AssetSonar Scrum Team" --status "in progress" --due-next-week
```

`--project` and `--status` match names case-insensitively (`in progress` finds `In Progress`), so you don't need exact server casing. `--due-within N` and `--due-next-week` compute the date range for you; `--due-after`/`--due-before` take explicit `YYYY-MM-DD` dates if you need a custom range.

## Multiple servers

```sh
rmine config add-profile work
rmine config use-profile work        # switch persistently
rmine --profile work issue list      # or override for one command
RMINE_PROFILE=work rmine issue list    # or via env var
rmine config list-profiles
```

## Commands

| Command | Description |
|---|---|
| `rmine whoami` | Show the user for the active profile's API key |
| `rmine project list` / `view <id>` | Browse projects |
| `rmine project categories <project>` | List a project's issue categories |
| `rmine issue list` | List issues (`--project`, `--status`, `--assignee`, `--tracker`, `--subject`, `--updated-after`, `--updated-before`, `--due-after`, `--due-before`, `--due-within`, `--due-next-week`, `--limit`, `--all`) |
| `rmine issue view <id>` | Show issue details |
| `rmine issue create` | Create an issue (`--project`, `--subject` required; `--description`, `--tracker`, `--priority`, `--category`, `--assignee`, `--field`) |
| `rmine issue update <id>` | Edit an issue (`--subject`, `--description`, `--tracker`, `--priority`, `--status`, `--category`, `--assignee`, `--field`) |
| `rmine issue close <id>` | Close an issue (`--status` to pick a specific closed status) |
| `rmine issue comment <id> <note>` | Add a comment |
| `rmine time log <issue-id>` | Log time (`--hours` required; `--date`, `--activity`, `--comment`) |
| `rmine time list` | List time entries (`--issue`, `--project`, `--user`, `--from`, `--to`) |
| `rmine time edit <id>` | Edit a time entry |
| `rmine time delete <id>` | Delete a time entry (prompts unless `-y`/`--force`) |
| `rmine config init` / `add-profile` / `use-profile` / `list-profiles` | Manage server profiles |

Every command accepts `-o json` for scripting-friendly output, and `--profile <name>` to target a specific server for that one call.

Assignees are set/filtered by numeric Redmine user ID (or the literal `me`); looking other users up by name isn't in scope yet.

Custom fields differ per Redmine instance (and sometimes per project/tracker), so they're set generically by numeric ID: `--field 12=staging`, repeatable to set several distinct fields. Passing the same ID more than once (`--field 11=16 --field 11=27`) instead sets that one field to multiple values, for checkbox/multi-select fields. Find a field's ID by inspecting an existing issue that has it set: `rmine issue view <id> -o json`.

`--category` is project-specific and matched case-insensitively by name; list a project's valid categories with `rmine project categories <project>`.

Different Redmine instances (and different projects/trackers within one) can require different mandatory fields — including custom fields. There's no reliable way to know these ahead of time (the field-configuration API is admin-only), so `rmine` just relies on Redmine's own validation: a `create`/`update` that's missing a required field returns the server's exact error (e.g. `redmine returned 422: Category cannot be blank`), naming what's missing so you can retry with it set.

`--subject` (and combining it with other filters) uses Redmine's advanced filter syntax under the hood, which doesn't default to open-only issues the way the plain filters do — expect closed issues in the results too unless you also pass `--status open`.

Shell completion is available via `rmine completion bash|zsh|fish` (see `rmine completion --help` for setup instructions).

## Development

```sh
go build ./...
go vet ./...
go test ./...
```

Releases are cut with [goreleaser](https://goreleaser.com/) on tag push (see `.github/workflows/release.yml`).
