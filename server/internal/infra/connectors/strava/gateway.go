// Package strava implementa o adaptador do conector Strava sobre a porta
// port.ProviderGateway: fluxo OAuth2 (autorização, troca de código e refresh) e
// consumo da API de atividades, normalizando o payload para a forma canônica do
// GAH. Todas as chamadas externas usam net/http com URLs base configuráveis,
// permitindo testes com httptest sem rede viva.
package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

const providerName = "strava"

// Endpoints reais da Strava (usados como default quando a Config não os sobrescreve).
const (
	defaultAuthURL    = "https://www.strava.com/oauth/authorize"
	defaultTokenURL   = "https://www.strava.com/oauth/token"
	defaultAPIBaseURL = "https://www.strava.com"
	defaultScope      = "read,activity:read_all"
)

// Config carrega os parâmetros do conector Strava. As URLs base são
// sobrescritíveis para testes; vazias caem nos endpoints reais.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	APIBaseURL   string
	Scope        string
}

// Gateway implementa port.ProviderGateway para a Strava.
type Gateway struct {
	cfg        Config
	httpClient *http.Client
}

var _ port.ProviderGateway = (*Gateway)(nil)

// NewGateway constrói o gateway aplicando defaults para campos vazios.
func NewGateway(cfg Config) *Gateway {
	if cfg.AuthURL == "" {
		cfg.AuthURL = defaultAuthURL
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = defaultTokenURL
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = defaultAPIBaseURL
	}
	if cfg.Scope == "" {
		cfg.Scope = defaultScope
	}
	return &Gateway{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (g *Gateway) Provider() string { return providerName }

// AuthURL monta a URL de autorização OAuth2 da Strava com o state embutido.
func (g *Gateway) AuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", g.cfg.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", g.cfg.RedirectURL)
	q.Set("scope", g.cfg.Scope)
	q.Set("approval_prompt", "auto")
	q.Set("state", state)
	return g.cfg.AuthURL + "?" + q.Encode()
}

// tokenResponse é o payload do endpoint /oauth/token da Strava.
type tokenResponse struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	Scope        string `json:"scope"`
	Athlete      struct {
		ID int64 `json:"id"`
	} `json:"athlete"`
}

func (g *Gateway) ExchangeCode(ctx context.Context, code string) (port.ProviderToken, error) {
	form := url.Values{}
	form.Set("client_id", g.cfg.ClientID)
	form.Set("client_secret", g.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	return g.requestToken(ctx, form)
}

func (g *Gateway) Refresh(ctx context.Context, refreshToken string) (port.ProviderToken, error) {
	form := url.Values{}
	form.Set("client_id", g.cfg.ClientID)
	form.Set("client_secret", g.cfg.ClientSecret)
	form.Set("refresh_token", refreshToken)
	form.Set("grant_type", "refresh_token")
	return g.requestToken(ctx, form)
}

func (g *Gateway) requestToken(ctx context.Context, form url.Values) (port.ProviderToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return port.ProviderToken{}, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return port.ProviderToken{}, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return port.ProviderToken{}, fmt.Errorf("strava token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return port.ProviderToken{}, fmt.Errorf("decode token response: %w", err)
	}

	tok := port.ProviderToken{
		Provider:     providerName,
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		Scope:        tr.Scope,
	}
	if tr.ExpiresAt > 0 {
		tok.ExpiresAt = time.Unix(tr.ExpiresAt, 0)
	}
	if tr.Athlete.ID != 0 {
		tok.AthleteID = strconv.FormatInt(tr.Athlete.ID, 10)
	}
	return tok, nil
}

// stravaActivity é o subconjunto do payload de /athlete/activities que mapeamos.
type stravaActivity struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	Type             string  `json:"type"`
	SportType        string  `json:"sport_type"`
	StartDate        string  `json:"start_date"`
	ElapsedTime      int64   `json:"elapsed_time"`
	MovingTime       int64   `json:"moving_time"`
	Distance         float64 `json:"distance"`
	AverageHeartrate float64 `json:"average_heartrate"`
	HasHeartrate     bool    `json:"has_heartrate"`
}

func (g *Gateway) FetchActivities(ctx context.Context, accessToken string, since time.Time) ([]port.ProviderActivity, error) {
	u, err := url.Parse(g.cfg.APIBaseURL + "/api/v3/athlete/activities")
	if err != nil {
		return nil, fmt.Errorf("build activities url: %w", err)
	}
	q := u.Query()
	q.Set("per_page", "100")
	if !since.IsZero() {
		q.Set("after", strconv.FormatInt(since.Unix(), 10))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build activities request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("activities request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("strava activities endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var raw []stravaActivity
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode activities: %w", err)
	}

	out := make([]port.ProviderActivity, 0, len(raw))
	for _, a := range raw {
		pa := port.ProviderActivity{
			Provider:       providerName,
			ExternalID:     strconv.FormatInt(a.ID, 10),
			Type:           string(mapType(a.SportType, a.Type)),
			Duration:       time.Duration(a.ElapsedTime) * time.Second,
			DistanceMeters: a.Distance,
			Name:           a.Name,
		}
		if a.HasHeartrate && a.AverageHeartrate > 0 {
			pa.AvgHeartRate = int(a.AverageHeartrate)
		}
		if t, err := time.Parse(time.RFC3339, a.StartDate); err == nil {
			pa.StartTime = t
		}
		out = append(out, pa)
	}
	return out, nil
}

// mapType traduz os tipos da Strava (sport_type tem prioridade sobre type) para
// os tipos canônicos do GAH. Tipos desconhecidos viram ActivityTypeOther.
func mapType(sportType, fallback string) valueobject.ActivityType {
	key := sportType
	if key == "" {
		key = fallback
	}
	switch strings.ToLower(key) {
	case "run", "trailrun", "virtualrun":
		return valueobject.ActivityTypeRunning
	case "ride", "virtualride", "ebikeride", "mountainbikeride", "gravelride", "handcycle":
		return valueobject.ActivityTypeCycling
	case "swim":
		return valueobject.ActivityTypeSwimming
	case "weighttraining":
		return valueobject.ActivityTypeWeightlifting
	case "yoga":
		return valueobject.ActivityTypeYoga
	case "hike":
		return valueobject.ActivityTypeHiking
	case "crossfit", "workout", "hiit":
		return valueobject.ActivityTypeCrossfit
	default:
		return valueobject.ActivityTypeOther
	}
}
