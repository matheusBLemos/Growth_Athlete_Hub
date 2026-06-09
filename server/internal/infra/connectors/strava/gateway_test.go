package strava_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/connectors/strava"
)

func newGateway(authURL, tokenURL, apiURL string) *strava.Gateway {
	return strava.NewGateway(strava.Config{
		ClientID:     "cid",
		ClientSecret: "secret",
		RedirectURL:  "https://gah.app/api/v1/connectors/strava/callback",
		AuthURL:      authURL,
		TokenURL:     tokenURL,
		APIBaseURL:   apiURL,
	})
}

func TestGateway_Provider(t *testing.T) {
	g := newGateway("", "", "")
	if g.Provider() != "strava" {
		t.Fatalf("provider = %q, want strava", g.Provider())
	}
}

func TestGateway_AuthURL(t *testing.T) {
	g := newGateway("https://strava.test/oauth/authorize", "", "")
	got := g.AuthURL("state-123")

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	q := u.Query()
	if q.Get("client_id") != "cid" {
		t.Errorf("client_id = %q", q.Get("client_id"))
	}
	if q.Get("state") != "state-123" {
		t.Errorf("state = %q", q.Get("state"))
	}
	if q.Get("response_type") != "code" {
		t.Errorf("response_type = %q", q.Get("response_type"))
	}
	if q.Get("redirect_uri") != "https://gah.app/api/v1/connectors/strava/callback" {
		t.Errorf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	if q.Get("scope") == "" {
		t.Error("scope should not be empty")
	}
	if !strings.HasPrefix(got, "https://strava.test/oauth/authorize?") {
		t.Errorf("auth url base = %q", got)
	}
}

const tokenJSON = `{
	"token_type": "Bearer",
	"access_token": "acc-token",
	"refresh_token": "ref-token",
	"expires_at": 1900000000,
	"scope": "activity:read_all",
	"athlete": {"id": 424242}
}`

func TestGateway_ExchangeCode(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/oauth/token" {
			t.Errorf("path = %s, want /oauth/token", r.URL.Path)
		}
		_ = r.ParseForm()
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tokenJSON))
	}))
	defer srv.Close()

	g := newGateway("", srv.URL+"/oauth/token", "")
	tok, err := g.ExchangeCode(context.Background(), "the-code")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}

	if gotForm.Get("client_id") != "cid" {
		t.Errorf("form client_id = %q", gotForm.Get("client_id"))
	}
	if gotForm.Get("client_secret") != "secret" {
		t.Errorf("form client_secret = %q", gotForm.Get("client_secret"))
	}
	if gotForm.Get("code") != "the-code" {
		t.Errorf("form code = %q", gotForm.Get("code"))
	}
	if gotForm.Get("grant_type") != "authorization_code" {
		t.Errorf("form grant_type = %q", gotForm.Get("grant_type"))
	}

	if tok.AccessToken != "acc-token" {
		t.Errorf("access_token = %q", tok.AccessToken)
	}
	if tok.RefreshToken != "ref-token" {
		t.Errorf("refresh_token = %q", tok.RefreshToken)
	}
	if tok.Provider != "strava" {
		t.Errorf("provider = %q", tok.Provider)
	}
	if tok.AthleteID != "424242" {
		t.Errorf("athlete_id = %q", tok.AthleteID)
	}
	if tok.Scope != "activity:read_all" {
		t.Errorf("scope = %q", tok.Scope)
	}
	if !tok.ExpiresAt.Equal(time.Unix(1900000000, 0)) {
		t.Errorf("expires_at = %v", tok.ExpiresAt)
	}
}

func TestGateway_Refresh(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tokenJSON))
	}))
	defer srv.Close()

	g := newGateway("", srv.URL+"/oauth/token", "")
	tok, err := g.Refresh(context.Background(), "old-refresh")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if gotForm.Get("grant_type") != "refresh_token" {
		t.Errorf("form grant_type = %q", gotForm.Get("grant_type"))
	}
	if gotForm.Get("refresh_token") != "old-refresh" {
		t.Errorf("form refresh_token = %q", gotForm.Get("refresh_token"))
	}
	if tok.AccessToken != "acc-token" {
		t.Errorf("access_token = %q", tok.AccessToken)
	}
}

func TestGateway_ExchangeCode_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Bad Request"}`))
	}))
	defer srv.Close()

	g := newGateway("", srv.URL+"/oauth/token", "")
	_, err := g.ExchangeCode(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error on non-200")
	}
}

const activitiesJSON = `[
	{
		"id": 111,
		"name": "Morning Run",
		"sport_type": "Run",
		"type": "Run",
		"start_date": "2025-06-01T06:00:00Z",
		"elapsed_time": 1800,
		"moving_time": 1700,
		"distance": 5000.5,
		"average_heartrate": 152.7,
		"has_heartrate": true
	},
	{
		"id": 222,
		"name": "Evening Ride",
		"sport_type": "Ride",
		"type": "Ride",
		"start_date": "2025-06-02T18:00:00Z",
		"elapsed_time": 3600,
		"moving_time": 3500,
		"distance": 20000,
		"average_heartrate": 0,
		"has_heartrate": false
	},
	{
		"id": 333,
		"name": "Unknown thing",
		"sport_type": "Kitesurf",
		"type": "Kitesurf",
		"start_date": "2025-06-03T10:00:00Z",
		"elapsed_time": 600,
		"moving_time": 600,
		"distance": 0,
		"has_heartrate": false
	}
]`

func TestGateway_FetchActivities(t *testing.T) {
	var gotAuth, gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(activitiesJSON))
	}))
	defer srv.Close()

	g := newGateway("", "", srv.URL)
	since := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
	acts, err := g.FetchActivities(context.Background(), "acc-token", since)
	if err != nil {
		t.Fatalf("FetchActivities: %v", err)
	}

	if gotAuth != "Bearer acc-token" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotPath != "/api/v3/athlete/activities" {
		t.Errorf("path = %q", gotPath)
	}
	if gotQuery.Get("after") == "" {
		t.Error("expected after query param")
	}

	if len(acts) != 3 {
		t.Fatalf("len(acts) = %d, want 3", len(acts))
	}

	run := acts[0]
	if run.ExternalID != "111" {
		t.Errorf("externalID = %q, want 111", run.ExternalID)
	}
	if run.Type != "running" {
		t.Errorf("type = %q, want running", run.Type)
	}
	if run.Provider != "strava" {
		t.Errorf("provider = %q", run.Provider)
	}
	if run.Duration != 1800*time.Second {
		t.Errorf("duration = %v, want 30m", run.Duration)
	}
	if run.AvgHeartRate != 152 {
		t.Errorf("avgHR = %d, want 152", run.AvgHeartRate)
	}
	if run.DistanceMeters != 5000.5 {
		t.Errorf("distance = %v", run.DistanceMeters)
	}

	ride := acts[1]
	if ride.Type != "cycling" {
		t.Errorf("ride type = %q, want cycling", ride.Type)
	}
	if ride.AvgHeartRate != 0 {
		t.Errorf("ride avgHR = %d, want 0", ride.AvgHeartRate)
	}

	other := acts[2]
	if other.Type != "other" {
		t.Errorf("other type = %q, want other", other.Type)
	}
}

func TestGateway_FetchActivities_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	g := newGateway("", "", srv.URL)
	_, err := g.FetchActivities(context.Background(), "acc-token", time.Time{})
	if err == nil {
		t.Fatal("expected error on non-200")
	}
}

var _ port.ProviderGateway = (*strava.Gateway)(nil)
