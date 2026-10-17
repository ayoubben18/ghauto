package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// runGh runs the real gh with the token of the account that can access the
// repo the command targets. Commands without a repo context pass through as-is.
func runGh(args []string) error {
	ghPath := realGh()
	if ghPath == "gh" {
		return fmt.Errorf("gh not found on PATH")
	}
	cmd := exec.Command(ghPath, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = os.Environ()

	// An explicit GH_TOKEN from the user always wins.
	if os.Getenv("GH_TOKEN") == "" {
		if host, repo := targetRepo(args); repo != "" {
			if res, _, err := resolve(host, repo); err == nil {
				cmd.Env = append(cmd.Env, "GH_TOKEN="+res.Token)
				if host != "github.com" {
					cmd.Env = append(cmd.Env, "GH_ENTERPRISE_TOKEN="+res.Token)
				}
			}
			// On failure, run gh untouched and let it report the error itself.
		}
	}

	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return err
}

// targetRepo works out which repo a gh command acts on, in gh's own order:
// --repo/-R flag, GH_REPO, a positional repo for `gh repo <cmd>`, then git remotes.
func targetRepo(args []string) (host, repo string) {
	for i, a := range args {
		var v string
		switch {
		case a == "-R" || a == "--repo":
			if i+1 < len(args) {
				v = args[i+1]
			}
		case strings.HasPrefix(a, "--repo="):
			v = strings.TrimPrefix(a, "--repo=")
		case strings.HasPrefix(a, "-R") && len(a) > 2:
			v = a[2:]
		}
		if v != "" {
			return parseRepoArg(v)
		}
	}
	if v := os.Getenv("GH_REPO"); v != "" {
		return parseRepoArg(v)
	}
	if len(args) >= 3 && args[0] == "repo" {
		switch args[1] {
		case "clone", "view", "fork", "edit", "delete", "archive", "unarchive", "rename", "sync":
			for _, a := range args[2:] {
				if !strings.HasPrefix(a, "-") {
					if h, r := parseRepoArg(a); r != "" {
						return h, r
					}
					break
				}
			}
		}
	}
	return repoFromRemotes()
}

// parseRepoArg accepts OWNER/REPO, HOST/OWNER/REPO, or a git URL.
func parseRepoArg(s string) (host, repo string) {
	if strings.Contains(s, "://") || strings.HasPrefix(s, "git@") {
		return parseRemoteURL(s)
	}
	parts := strings.Split(strings.Trim(s, "/"), "/")
	switch len(parts) {
	case 2:
		return "github.com", parts[0] + "/" + strings.TrimSuffix(parts[1], ".git")
	case 3:
		return parts[0], parts[1] + "/" + strings.TrimSuffix(parts[2], ".git")
	}
	return "", ""
}

// parseRemoteURL handles https://host/owner/repo(.git) and git@host:owner/repo(.git).
func parseRemoteURL(raw string) (host, repo string) {
	if rest, ok := strings.CutPrefix(raw, "git@"); ok {
		h, path, ok := strings.Cut(rest, ":")
		if !ok {
			return "", ""
		}
		return h, repoFromPath(path)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", ""
	}
	if u.Scheme == "ssh" {
		return u.Hostname(), repoFromPath(u.Path)
	}
	return u.Host, repoFromPath(u.Path)
}

// repoFromRemotes mirrors gh's remote preference: upstream, github, origin, then any.
func repoFromRemotes() (host, repo string) {
	out, err := exec.Command("git", "remote").Output()
	if err != nil {
		return "", ""
	}
	remotes := strings.Fields(string(out))
	ordered := []string{"upstream", "github", "origin"}
	ordered = append(ordered, remotes...)
	for _, name := range ordered {
		for _, r := range remotes {
			if r != name {
				continue
			}
			u, err := exec.Command("git", "remote", "get-url", r).Output()
			if err != nil {
				continue
			}
			if h, rp := parseRemoteURL(strings.TrimSpace(string(u))); rp != "" {
				return h, rp
			}
		}
	}
	return "", ""
}
