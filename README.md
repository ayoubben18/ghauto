# ghauto

Push to any GitHub repo with whichever `gh` account has access. No more `gh auth switch`.

ghauto is a git credential helper. When git needs credentials for `github.com/owner/repo`,
ghauto asks the GitHub API which of your `gh` accounts can access it, picks the one with the
highest permission, caches the choice, and hands git that account's token.

## Install

```sh
go install github.com/ayoubben21/ghauto@latest
ghauto setup       # makes ghauto git's credential helper for github.com
```

Requires `gh` with your accounts logged in (`gh auth login` for each). HTTPS remotes only.

## Commands

```
ghauto which [remote]   show which account a repo resolves to
ghauto forget [repo]    clear a cached choice (or all)
ghauto uninstall        restore gh's credential helper
```
