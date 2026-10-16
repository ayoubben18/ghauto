package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Permission levels, higher is better.
const (
	permNone = iota
	permRead
	permTriage
	permPush
	permMaintain
	permAdmin
)

var permNames = map[int]string{
	permNone: "none", permRead: "read", permTriage: "triage",
	permPush: "push", permMaintain: "maintain", permAdmin: "admin",
}

type probeResult struct {
	Login string
	Token string
	Perm  int
	Err   error
}

// probe asks the GitHub API what access one token has on owner/repo.
func probe(host, repo, token string) (int, error) {
	api := "https://api.github.com"
	if host != "github.com" {
		api = "https://" + host + "/api/v3"
	}
	req, _ := http.NewRequest("GET", api+"/repos/"+repo, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return permNone, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return permNone, nil
	}
	if resp.StatusCode != http.StatusOK {
		if sso := resp.Header.Get("X-GitHub-SSO"); sso != "" {
			return permNone, fmt.Errorf("SSO authorization required: %s", sso)
		}
		return permNone, fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	var body struct {
		Permissions map[string]bool `json:"permissions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return permNone, err
	}
	p := body.Permissions
	switch {
	case p["admin"]:
		return permAdmin, nil
	case p["maintain"]:
		return permMaintain, nil
	case p["push"]:
		return permPush, nil
	case p["triage"]:
		return permTriage, nil
	default:
		// A 200 means we can at least read it (e.g. a public repo).
		return permRead, nil
	}
}

// probeAll checks every account in parallel.
func probeAll(host, repo string, accounts []account) []probeResult {
	results := make([]probeResult, len(accounts))
	var wg sync.WaitGroup
	for i, a := range accounts {
		wg.Add(1)
		go func(i int, a account) {
			defer wg.Done()
			r := probeResult{Login: a.Login}
			r.Token, r.Err = ghToken(host, a.Login)
			if r.Err == nil {
				r.Perm, r.Err = probe(host, repo, r.Token)
			}
			results[i] = r
		}(i, a)
	}
	wg.Wait()
	return results
}

// best picks the account with the highest permission.
// Ties go to the gh-active account, then to list order.
func best(results []probeResult, accounts []account) *probeResult {
	var pick *probeResult
	for i := range results {
		r := &results[i]
		if r.Err != nil || r.Perm == permNone {
			continue
		}
		if pick == nil || r.Perm > pick.Perm || (r.Perm == pick.Perm && accounts[i].Active) {
			pick = r
		}
	}
	return pick
}

// repoFromPath turns a credential path like "owner/repo.git/info/lfs"
// into "owner/repo".
func repoFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "/" + strings.TrimSuffix(parts[1], ".git")
}

type resolution struct {
	Login  string
	Token  string
	Perm   int
	Cached bool
}

// resolve finds the account to use for host/repo, using the cache when possible.
func resolve(host, repo string) (*resolution, []probeResult, error) {
	c := loadCache()
	key := host + "/" + repo
	if login, ok := c[key]; ok {
		// Fast path: skip `gh auth status`, which validates every token online.
		if token, err := ghToken(host, login); err == nil {
			return &resolution{Login: login, Token: token, Cached: true}, nil, nil
		}
		// Cached account was logged out; fall through and re-probe.
	}

	accounts, err := ghAccounts(host)
	if err != nil {
		return nil, nil, err
	}
	if len(accounts) == 0 {
		return nil, nil, fmt.Errorf("no gh accounts logged in for %s (run: gh auth login)", host)
	}

	results := probeAll(host, repo, accounts)
	pick := best(results, accounts)
	if pick == nil {
		return nil, results, fmt.Errorf("none of your accounts can access %s", repo)
	}
	c[key] = pick.Login
	saveCache(c)
	return &resolution{Login: pick.Login, Token: pick.Token, Perm: pick.Perm}, results, nil
}
