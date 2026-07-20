# rdm

A command-line client for [Redmine](https://www.redmine.org/) — issues, projects, and time logging, without leaving the terminal. Supports multiple Redmine servers via named profiles.

## Install

Download a binary from the [Releases](https://github.com/mehboobali98/rdm/releases) page, or build from source:

```sh
go install github.com/mehboobali98/rdm/cmd/rdm@latest
```

`go install` puts the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`) — make sure that's on your `PATH`.

If you use oh-my-zsh's `rails` plugin, note that it aliases `rdm` to `rails db:migrate`, which will shadow this binary. Run `unalias rdm` (or remove `rails` from your `plugins=(...)` list) to use this CLI instead.

## Getting started

```sh
rdm config init
```

You'll be asked for your Redmine server URL and a personal API key (found under *My account → API access key* in Redmine). This saves a `default` profile and verifies it works.

```sh
rdm whoami
rdm project list
rdm issue list --project myproject --status open
rdm time log 1234 --hours 1.5 --activity Development --comment "Fixed the login bug"
```

## Multiple servers

```sh
rdm config add-profile work
rdm config use-profile work        # switch persistently
rdm --profile work issue list      # or override for one command
RDM_PROFILE=work rdm issue list    # or via env var
rdm config list-profiles
```

## Commands

| Command | Description |
|---|---|
| `rdm whoami` | Show the user for the active profile's API key |
| `rdm project list` / `view <id>` | Browse projects |
| `rdm issue list` | List issues (`--project`, `--status`, `--assignee`, `--tracker`, `--limit`, `--all`) |
| `rdm issue view <id>` | Show issue details |
| `rdm issue create` | Create an issue (`--project`, `--subject` required; `--description`, `--tracker`, `--priority`, `--assignee`) |
| `rdm issue update <id>` | Edit an issue |
| `rdm issue close <id>` | Close an issue (`--status` to pick a specific closed status) |
| `rdm issue comment <id> <note>` | Add a comment |
| `rdm time log <issue-id>` | Log time (`--hours` required; `--date`, `--activity`, `--comment`) |
| `rdm time list` | List time entries (`--issue`, `--project`, `--user`, `--from`, `--to`) |
| `rdm time edit <id>` | Edit a time entry |
| `rdm time delete <id>` | Delete a time entry (prompts unless `-y`/`--force`) |
| `rdm config init` / `add-profile` / `use-profile` / `list-profiles` | Manage server profiles |

Every command accepts `-o json` for scripting-friendly output, and `--profile <name>` to target a specific server for that one call.

Assignees are set/filtered by numeric Redmine user ID; looking users up by name isn't in scope yet.

Shell completion is available via `rdm completion bash|zsh|fish` (see `rdm completion --help` for setup instructions).

## Development

```sh
go build ./...
go vet ./...
go test ./...
```

Releases are cut with [goreleaser](https://goreleaser.com/) on tag push (see `.github/workflows/release.yml`).
