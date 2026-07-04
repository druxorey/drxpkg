// Package pkgmgr provides package management backend operations for AUR and local Pacman (ALPM) databases.
package pkgmgr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)


func TestSearchAur(t *testing.T) {
	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") != "search" {
			t.Errorf("expected search type, got %s", r.URL.Query().Get("type"))
		}
		if r.URL.Query().Get("arg") != "my-pkg" {
			t.Errorf("expected arg my-pkg, got %s", r.URL.Query().Get("arg"))
		}

		response := SearchResults{
			Version:     5,
			Type:        "search",
			Resultcount: 2,
			Results: []InfoRecord{
				{
					Name:         "my-pkg-beta",
					NumVotes:     5,
					Popularity:   0.5,
					LastModified: 1000,
				},
				{
					Name:         "my-pkg",
					NumVotes:     100,
					Popularity:   2.5,
					LastModified: 2000,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	pkgs, err := SearchAur(context.Background(), server.URL, "my-pkg", 1000, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pkgs) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(pkgs))
	}

	// Should be sorted by NumVotes desc: my-pkg first, then my-pkg-beta
	if pkgs[0].Name != "my-pkg" {
		t.Errorf("expected my-pkg first (sorted by votes), got %s", pkgs[0].Name)
	}
	if pkgs[1].Name != "my-pkg-beta" {
		t.Errorf("expected my-pkg-beta second, got %s", pkgs[1].Name)
	}
}


func TestInfoAur(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("type") != "info" {
			t.Errorf("expected info type, got %s", r.FormValue("type"))
		}
		args := r.Form["arg[]"]
		if len(args) != 1 || args[0] != "my-pkg" {
			t.Errorf("expected arg[] to contain 'my-pkg', got %v", args)
		}

		response := SearchResults{
			Version:     5,
			Type:        "info",
			Resultcount: 1,
			Results: []InfoRecord{
				{
					Name:         "my-pkg",
					Description:  "Mocked package info",
					Version:      "1.0.0",
					URL:          "https://example.com",
					License:      []string{"GPL"},
					Maintainer:   "tester",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	res := InfoAur(server.URL, 1000, "my-pkg")
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}

	if len(res.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res.Results))
	}

	if res.Results[0].Name != "my-pkg" || res.Results[0].Description != "Mocked package info" {
		t.Errorf("unexpected record contents: %+v", res.Results[0])
	}
}
