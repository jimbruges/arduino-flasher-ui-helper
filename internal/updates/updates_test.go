package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReleaseSelection(t *testing.T) {
	inventory := `[
 {"tag_name":"v0.1.9-preview.1","prerelease":true},
 {"tag_name":"v0.1.2-preview.2","prerelease":true},
 {"tag_name":"v0.1.10-preview.1","prerelease":true},
 {"tag_name":"v0.1.2","prerelease":false},
 {"tag_name":"v9.0.0","draft":true},
 {"tag_name":"not-a-version"}
 ]`
	for _, tc := range []struct {
		current, latest       string
		available, comparable bool
	}{
		{"v0.1.1-preview.1", "v0.1.10-preview.1", true, true},
		{"v0.1.10-preview.1", "v0.1.10-preview.1", false, true},
		{"v0.1.11-preview.1", "v0.1.10-preview.1", false, true},
		{"v0.1.1", "v0.1.2", true, true},
		{"v0.1.2", "v0.1.2", false, true},
		{"development", "v0.1.10-preview.1", false, false},
	} {
		t.Run(tc.current, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("User-Agent") == "" || r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
					t.Error("missing GitHub request headers")
				}
				w.Write([]byte(inventory))
			}))
			defer server.Close()
			info, err := check(context.Background(), server.Client(), server.URL, tc.current)
			if err != nil || info.Latest != tc.latest || info.Available != tc.available || info.Comparable != tc.comparable {
				t.Fatalf("%+v %v", info, err)
			}
		})
	}
}

func TestStableSupersedesItsPreview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"tag_name":"v0.1.2-preview.10","prerelease":true},{"tag_name":"v0.1.2"}]`))
	}))
	defer server.Close()
	info, err := check(context.Background(), server.Client(), server.URL, "v0.1.2-preview.10")
	if err != nil || !info.Available || info.Latest != "v0.1.2" || info.Preview {
		t.Fatalf("%+v %v", info, err)
	}
}

func TestGitHubErrors(t *testing.T) {
	for _, tc := range []struct {
		status     int
		body, want string
	}{
		{403, `{}`, "limited"}, {429, `{}`, "limited"}, {500, `{}`, "HTTP 500"},
		{200, `not JSON`, "could not read"}, {200, `[]`, "no published"},
		{200, `[{"tag_name":"v0.1.2-preview.1","prerelease":true}]`, "no published"},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
		_, err := check(context.Background(), server.Client(), server.URL, "v0.1.1")
		server.Close()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("want %q, got %v", tc.want, err)
		}
	}
}

func TestCancellationAndOffline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := check(ctx, &http.Client{Timeout: time.Second}, "https://api.github.com/", "v0.1.1"); err == nil {
		t.Fatal("ignored cancellation")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	if _, err := check(context.Background(), server.Client(), server.URL, "v0.1.1"); err == nil {
		t.Fatal("ignored offline server")
	}
}

func TestReleaseLinks(t *testing.T) {
	link, err := ReleaseURL("v0.1.2-preview.1")
	if err != nil || link != ReleasesURL+"/tag/v0.1.2-preview.1" {
		t.Fatalf("%s %v", link, err)
	}
	for _, tag := range []string{"https://example.com", "../../settings", "v1.0.0\""} {
		if _, err := ReleaseURL(tag); err == nil {
			t.Fatal("accepted", tag)
		}
	}
}
