// ghauto is a git credential helper that picks the right gh account
// for each GitHub repo automatically.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const usage = `ghauto - push to any GitHub repo with whichever gh account has access

Usage:
  ghauto setup            Use ghauto as git's credential helper for github.com
  ghauto uninstall        Restore gh's default credential helper
  ghauto gh <args>        Run gh as the account that can access the target repo
  ghauto which [remote]   Show which account will be used (default remote: origin)
  ghauto forget [repo]    Drop a cached choice (owner/repo), or all with no argument
  ghauto credential get|store|erase   (called by git)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "credential":
		if len(os.Args) < 3 {
			err = fmt.Errorf("missing credential operation")
			break
		}
		err = credential(os.Args[2])
	case "gh":
		err = runGh(os.Args[2:])
	case "setup":
		err = setup()
	case "uninstall":
		err = uninstall()
	case "which":
		remote := "origin"
		if len(os.Args) > 2 {
			remote = os.Args[2]
		}
		err = which(remote)
	case "forget":
		c := loadCache()
		if len(os.Args) > 2 {
			delete(c, "github.com/"+os.Args[2])
		} else {
			c = cache{}
		}
		saveCache(c)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ghauto:", err)
		os.Exit(1)
	}
}

// credential implements git's credential helper protocol.
func credential(op string) error {
	in := map[string]string{}
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			in[k] = v
		}
	}
	host := in["host"]
	if in["protocol"] != "https" || host == "" {
		return nil
	}

	switch op {
	case "get":
		repo := repoFromPath(in["path"])
		if repo == "" {
			// No path (credential.useHttpPath is off): use the active gh account.
			return emitActive(host)
		}
		res, results, err := resolve(host, repo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ghauto: %v\n", err)
			for _, r := range results {
				if r.Err != nil {
					fmt.Fprintf(os.Stderr, "  %s: %v\n", r.Login, r.Err)
				} else {
					fmt.Fprintf(os.Stderr, "  %s: no access (repo is private or doesn't exist)\n", r.Login)
				}
			}
			// Tell git to stop instead of prompting for a password.
			fmt.Println("quit=true")
			return nil
		}
		fmt.Printf("username=%s\npassword=%s\n", res.Login, res.Token)
	case "erase":
		// Git calls erase when the credentials we gave were rejected.
		if repo := repoFromPath(in["path"]); repo != "" {
			c := loadCache()
			delete(c, host+"/"+repo)
			saveCache(c)
		}
	case "store":
		// Nothing to store: tokens live in gh's keyring.
	}
	return nil
}

func emitActive(host string) error {
	accounts, err := ghAccounts(host)
	if err != nil {
		return err
	}
	for _, a := range accounts {
		if a.Active {
			token, err := ghToken(host, a.Login)
			if err != nil {
				return err
			}
			fmt.Printf("username=%s\npassword=%s\n", a.Login, token)
			return nil
		}
	}
	return nil
}

func which(remote string) error {
	out, err := exec.Command("git", "remote", "get-url", "--push", remote).Output()
	if err != nil {
		return fmt.Errorf("no remote %q in this repo", remote)
	}
	raw := strings.TrimSpace(string(out))
	host, repo := parseRemoteURL(raw)
	if repo == "" {
		return fmt.Errorf("can't parse remote %q: %s", remote, raw)
	}
	res, results, err := resolve(host, repo)
	if err != nil {
		for _, r := range results {
			if r.Err != nil {
				fmt.Fprintf(os.Stderr, "  %s: %v\n", r.Login, r.Err)
			}
		}
		return err
	}
	if res.Cached {
		fmt.Printf("%s -> %s (cached)\n", repo, res.Login)
	} else {
		fmt.Printf("%s -> %s (%s)\n", repo, res.Login, permNames[res.Perm])
	}
	return nil
}

func gitConfig(args ...string) error {
	cmd := exec.Command("git", append([]string{"config", "--global"}, args...)...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func setup() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	const key = "credential.https://github.com.helper"
	// Replace any existing github.com helpers (e.g. gh's) with ours.
	_ = gitConfig("--unset-all", key)
	if err := gitConfig("--add", key, ""); err != nil {
		return err
	}
	if err := gitConfig("--add", key, "!"+self+" credential"); err != nil {
		return err
	}
	if err := gitConfig("credential.https://github.com.useHttpPath", "true"); err != nil {
		return err
	}
	fmt.Println("ghauto is now git's credential helper for github.com.")
	fmt.Println("Undo with: ghauto uninstall")
	return nil
}

func uninstall() error {
	_ = gitConfig("--unset-all", "credential.https://github.com.helper")
	_ = gitConfig("--unset", "credential.https://github.com.useHttpPath")
	cmd := exec.Command("gh", "auth", "setup-git")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restore gh helper: %w", err)
	}
	fmt.Println("Restored gh's credential helper.")
	return nil
}
