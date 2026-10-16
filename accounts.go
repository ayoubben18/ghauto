package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type account struct {
	Login  string
	Active bool
}

// ghAccounts lists the accounts logged into gh for a host.
func ghAccounts(host string) ([]account, error) {
	out, err := exec.Command("gh", "auth", "status", "--hostname", host, "--json", "hosts").Output()
	if err != nil {
		return nil, fmt.Errorf("gh auth status: %w", err)
	}
	var status struct {
		Hosts map[string][]struct {
			Login  string `json:"login"`
			Active bool   `json:"active"`
			State  string `json:"state"`
		} `json:"hosts"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("parse gh auth status: %w", err)
	}
	var accounts []account
	for _, a := range status.Hosts[host] {
		if a.State == "success" {
			accounts = append(accounts, account{Login: a.Login, Active: a.Active})
		}
	}
	return accounts, nil
}

// ghToken returns the stored token for one gh account.
func ghToken(host, login string) (string, error) {
	out, err := exec.Command("gh", "auth", "token", "--hostname", host, "--user", login).Output()
	if err != nil {
		return "", fmt.Errorf("gh auth token for %s: %w", login, err)
	}
	return strings.TrimSpace(string(out)), nil
}
