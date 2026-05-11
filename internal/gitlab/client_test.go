package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	_, err := NewClient("gitlab.com", "t")
	if err == nil {
		t.Fatal("expected error without scheme")
	}
	c, err := NewClient("https://gitlab.example.com", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.perPage != 100 {
		t.Fatal(c)
	}
}

func TestResolveProjects_singleProject(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/api/v4/projects/g%2Fp":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                  42,
				"path_with_namespace": "g/p",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	c, err := NewClient(ts.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	c.http = ts.Client()

	refs, err := c.ResolveProjects(context.Background(), nil, []string{"g/p"})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].ID != 42 || refs[0].PathWithNamespace != "g/p" {
		t.Fatalf("%+v", refs)
	}
}

func TestResolveProjects_groupPagination(t *testing.T) {
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v4/groups/mygrp%2Fsub%2Fg/projects":
			calls++
			per := r.URL.Query().Get("per_page")
			if per != "100" {
				http.Error(w, per, http.StatusBadRequest)
				return
			}
			page := r.URL.Query().Get("page")
			if page == "1" {
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"id": 1, "path_with_namespace": "mygrp/sub/g/a"},
					{"id": 2, "path_with_namespace": "mygrp/sub/g/b"},
				})
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	c, err := NewClient(ts.URL, "t")
	if err != nil {
		t.Fatal(err)
	}
	c.http = ts.Client()

	refs, err := c.ResolveProjects(context.Background(), []string{"mygrp/sub/g"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("group API calls: %d", calls)
	}
	if len(refs) != 2 {
		t.Fatal(refs)
	}
}

func TestGet_errorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer ts.Close()

	c, err := NewClient(ts.URL, "t")
	if err != nil {
		t.Fatal(err)
	}
	c.http = ts.Client()

	_, err = c.get(context.Background(), "/projects/1", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
