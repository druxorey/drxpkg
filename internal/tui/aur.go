package tui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const DefaultAurRpcUrl = "https://aur.archlinux.org/rpc"

func SearchAur(aurUrl, term string, timeoutMs int, maxResults int) ([]Package, error) {
	packages := []Package{}
	if aurUrl == "" {
		aurUrl = DefaultAurRpcUrl
	}

	client := http.Client{
		Timeout: time.Duration(timeoutMs) * time.Millisecond,
	}

	req, err := http.NewRequest("GET", aurUrl+"?v=5&type=search&by=name&arg="+url.QueryEscape(term), nil)
	if err != nil {
		return packages, err
	}
	req.Header.Set("User-Agent", "drxpkg")

	resp, err := client.Do(req)
	if err != nil {
		return packages, err
	}
	defer resp.Body.Close()

	var s SearchResults
	if err = json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return packages, err
	}

	if s.Error != "" {
		return packages, errors.New(s.Error)
	}

	sort.Slice(s.Results, func(i, j int) bool {
		return s.Results[i].Name < s.Results[j].Name
	})

	for _, pkg := range s.Results {
		packages = append(packages, Package{
			Name:         pkg.Name,
			Source:       "AUR",
			LastModified: pkg.LastModified,
			Popularity:   pkg.Popularity,
			Votes:        pkg.NumVotes,
		})
		if len(packages) >= maxResults {
			break
		}
	}

	return packages, nil
}

func InfoAur(aurURL string, timeoutMs int, pkgs ...string) SearchResults {
	if aurURL == "" {
		aurURL = DefaultAurRpcUrl
	}

	client := http.Client{
		Timeout: time.Duration(timeoutMs) * time.Millisecond,
	}

	data := url.Values{}
	data.Add("v", "5")
	data.Add("type", "info")
	for _, p := range pkgs {
		data.Add("arg[]", p)
	}

	req, err := http.NewRequest("POST", aurURL, strings.NewReader(data.Encode()))
	if err != nil {
		return SearchResults{Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "drxpkg")

	resp, err := client.Do(req)
	if err != nil {
		return SearchResults{Error: err.Error()}
	}
	defer resp.Body.Close()

	var p SearchResults
	if err = json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return SearchResults{Error: err.Error()}
	}

	for i := 0; i < len(p.Results); i++ {
		p.Results[i].Source = "AUR"
	}

	return p
}
