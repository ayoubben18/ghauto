# ghauto

Push to any GitHub repo with whichever `gh` account has access. No more `gh auth switch`.

If you have several GitHub accounts (personal, work, client...), `git push` only works for
repos the currently active account can access. ghauto fixes that: it picks the right account
for each repo automatically, so plain `git push`, `git pull` and `git clone` just work.

## How it works

ghauto is a [git credential helper](https://git-scm.com/docs/gitcredentials). It never wraps
`git push`, so every git flag (`--force-with-lease`, `--no-verify`, `-u`...) behaves as usual.

1. Git needs credentials for `github.com/owner/repo` and asks ghauto.
2. ghauto asks the GitHub API, with each of your `gh` accounts in parallel, what access it has on that repo.
3. It picks the account with the highest permission and caches the choice (`~/.config/ghauto/cache.json`).
4. It hands git that account's token. Tokens stay in `gh`'s keyring; ghauto stores none.

If no account can access the repo, git stops with a clear error instead of prompting for a password.
If GitHub rejects a cached account later, the cache entry is dropped and the next push re-checks.

## Requirements

- [GitHub CLI](https://cli.github.com) (`gh`) 2.40 or newer
- Go 1.22+ (to build)
- HTTPS remotes (`https://github.com/owner/repo.git`)

## Setup

### 1. Log every account into `gh`

```sh
gh auth login          # repeat once per account
gh auth status         # should list all of them
```

### 2. Install ghauto

With Go:

```sh
go install github.com/ayoubben21/ghauto@latest
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`.

Or clone and build:

```sh
git clone https://github.com/ayoubben21/ghauto.git
cd ghauto
go build -o ghauto .
sudo mv ghauto /usr/local/bin/   # or anywhere on your PATH
```

### 3. Turn it on

```sh
ghauto setup
```

This replaces `gh`'s credential helper for `github.com` in your global git config with:

```ini
[credential "https://github.com"]
    helper =
    helper = !/path/to/ghauto credential
    useHttpPath = true
```

`useHttpPath` makes git tell the helper which repo it's talking to.

> If you move the binary later, run `ghauto setup` again: it records the binary's absolute path.

### 4. Check it

Inside any repo:

```sh
ghauto which
# owner/repo -> your-work-account (push)
```

Then just `git push`.

### Using SSH remotes?

ghauto only handles HTTPS. Switch a repo's remote with:

```sh
git remote set-url origin https://github.com/owner/repo.git
```

## Commands

| Command | What it does |
| --- | --- |
| `ghauto setup` | Make ghauto git's credential helper for github.com |
| `ghauto which [remote]` | Show which account a repo resolves to (default `origin`) |
| `ghauto forget [owner/repo]` | Clear one cached choice, or all of them |
| `ghauto uninstall` | Restore `gh`'s credential helper |

## Notes

- For public repos you can only read, ghauto uses your active `gh` account.
- Orgs with SAML SSO: if a token isn't authorized for the org, ghauto shows the authorization link.
- Commit author email isn't switched per account; use git's `includeIf` for that.

## Uninstall

```sh
ghauto uninstall
rm -rf ~/.config/ghauto
```

## License

MIT
