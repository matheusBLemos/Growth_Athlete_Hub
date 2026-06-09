package usecase_test

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type mockActivityRepo struct {
	activities map[string]*entity.Activity
	byExternal map[string]*entity.Activity
	saveCalled int
	saveErr    error
}

func newMockActivityRepo() *mockActivityRepo {
	return &mockActivityRepo{
		activities: make(map[string]*entity.Activity),
		byExternal: make(map[string]*entity.Activity),
	}
}

func (m *mockActivityRepo) Save(_ context.Context, a *entity.Activity) error {
	m.saveCalled++
	if m.saveErr != nil {
		return m.saveErr
	}
	m.activities[a.ID] = a
	if a.ExternalID != "" {
		m.byExternal[a.ExternalID] = a
	}
	return nil
}

func (m *mockActivityRepo) FindByID(_ context.Context, id string) (*entity.Activity, error) {
	a, ok := m.activities[id]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (m *mockActivityRepo) FindByUserID(_ context.Context, userID string, from, to time.Time) ([]*entity.Activity, error) {
	var result []*entity.Activity
	for _, a := range m.activities {
		if a.UserID == userID && !a.Date.Before(from) && !a.Date.After(to) {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockActivityRepo) FindByExternalID(_ context.Context, externalID string) (*entity.Activity, error) {
	a, ok := m.byExternal[externalID]
	if !ok {
		return nil, nil
	}
	return a, nil
}

var _ port.ActivityRepository = (*mockActivityRepo)(nil)

type mockMetricRepo struct {
	metrics    []*entity.Metric
	saveCalled int
	findCalls  int
	saveErr    error
}

func newMockMetricRepo() *mockMetricRepo {
	return &mockMetricRepo{}
}

func (m *mockMetricRepo) Save(_ context.Context, metric *entity.Metric) error {
	m.saveCalled++
	if m.saveErr != nil {
		return m.saveErr
	}
	m.metrics = append(m.metrics, metric)
	return nil
}

func (m *mockMetricRepo) FindByUserIDAndType(_ context.Context, userID string, metricType valueobject.MetricType, from, to time.Time) ([]*entity.Metric, error) {
	m.findCalls++
	var result []*entity.Metric
	for _, met := range m.metrics {
		if met.UserID == userID && met.Type == metricType && !met.Date.Before(from) && !met.Date.After(to) {
			result = append(result, met)
		}
	}
	return result, nil
}

func (m *mockMetricRepo) FindLatestByUserIDAndType(_ context.Context, userID string, metricType valueobject.MetricType, limit int) ([]*entity.Metric, error) {
	var result []*entity.Metric
	for _, met := range m.metrics {
		if met.UserID == userID && met.Type == metricType {
			result = append(result, met)
		}
	}
	if len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result, nil
}

var _ port.MetricRepository = (*mockMetricRepo)(nil)

type mockInsightRepo struct {
	insights   []*entity.Insight
	saveCalled int
}

func newMockInsightRepo() *mockInsightRepo {
	return &mockInsightRepo{}
}

func (m *mockInsightRepo) Save(_ context.Context, i *entity.Insight) error {
	m.saveCalled++
	m.insights = append(m.insights, i)
	return nil
}

func (m *mockInsightRepo) SaveAll(_ context.Context, insights []*entity.Insight) error {
	m.saveCalled++
	m.insights = append(m.insights, insights...)
	return nil
}

func (m *mockInsightRepo) FindByUserID(_ context.Context, userID string, from, to time.Time) ([]*entity.Insight, error) {
	var result []*entity.Insight
	for _, i := range m.insights {
		if i.UserID == userID && !i.Date.Before(from) && !i.Date.After(to) {
			result = append(result, i)
		}
	}
	return result, nil
}

var _ port.InsightRepository = (*mockInsightRepo)(nil)

type mockInsightEvaluator struct {
	result []*entity.Insight
	err    error
}

func (m *mockInsightEvaluator) Evaluate(_ context.Context, _ string, _ []*entity.Metric) ([]*entity.Insight, error) {
	return m.result, m.err
}

var _ port.InsightEvaluator = (*mockInsightEvaluator)(nil)

type mockEventPublisher struct {
	events []port.Event
	err    error
}

func (m *mockEventPublisher) Publish(_ context.Context, event port.Event) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

var _ port.EventPublisher = (*mockEventPublisher)(nil)

type mockUserRepo struct {
	byID       map[string]*entity.User
	byEmail    map[string]*entity.User
	saveCalled int
	saveErr    error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		byID:    make(map[string]*entity.User),
		byEmail: make(map[string]*entity.User),
	}
}

func (m *mockUserRepo) Save(_ context.Context, u *entity.User) error {
	m.saveCalled++
	if m.saveErr != nil {
		return m.saveErr
	}
	m.byID[u.ID] = u
	m.byEmail[u.Email] = u
	return nil
}

func (m *mockUserRepo) FindByID(_ context.Context, id string) (*entity.User, error) {
	u, ok := m.byID[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockUserRepo) FindByEmail(_ context.Context, email string) (*entity.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

var _ port.UserRepository = (*mockUserRepo)(nil)

// mockHasher é um hasher determinístico de teste: hash = "hashed:" + plain.
type mockHasher struct {
	hashErr error
}

func (m *mockHasher) Hash(plain string) (string, error) {
	if m.hashErr != nil {
		return "", m.hashErr
	}
	return "hashed:" + plain, nil
}

func (m *mockHasher) Compare(hash, plain string) error {
	if hash != "hashed:"+plain {
		return errors.New("mismatch")
	}
	return nil
}

var _ port.PasswordHasher = (*mockHasher)(nil)

type mockTokenIssuer struct {
	issueErr error
}

func (m *mockTokenIssuer) Issue(userID string) (string, error) {
	if m.issueErr != nil {
		return "", m.issueErr
	}
	return "token:" + userID, nil
}

func (m *mockTokenIssuer) Parse(token string) (string, error) {
	const prefix = "token:"
	if len(token) <= len(prefix) || token[:len(prefix)] != prefix {
		return "", errors.New("invalid token")
	}
	return token[len(prefix):], nil
}

var _ port.TokenIssuer = (*mockTokenIssuer)(nil)

// fakeCache é um cache em memória para testes da lógica cache-aside.
// getErr/setErr permitem simular falhas de cache (resiliência).
type fakeCache struct {
	mu       sync.Mutex
	store    map[string][]byte
	getCalls int
	setCalls int
	getErr   error
	setErr   error
}

func newFakeCache() *fakeCache {
	return &fakeCache{store: make(map[string][]byte)}
}

func (f *fakeCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCalls++
	if f.getErr != nil {
		return nil, false, f.getErr
	}
	v, ok := f.store[key]
	if !ok {
		return nil, false, nil
	}
	// Devolve uma cópia para evitar aliasing acidental nos testes.
	cp := make([]byte, len(v))
	copy(cp, v)
	return cp, true, nil
}

func (f *fakeCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setCalls++
	if f.setErr != nil {
		return f.setErr
	}
	cp := make([]byte, len(value))
	copy(cp, value)
	f.store[key] = cp
	return nil
}

func (f *fakeCache) Delete(_ context.Context, keys ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, k := range keys {
		delete(f.store, k)
	}
	return nil
}

var _ port.Cache = (*fakeCache)(nil)

// mockProviderGateway é um gateway de provedor externo controlável para testes.
type mockProviderGateway struct {
	provider        string
	exchanged       port.ProviderToken
	exchangeErr     error
	refreshed       port.ProviderToken
	refreshErr      error
	refreshCalled   bool
	activities      []port.ProviderActivity
	fetchErr        error
	usedAccessToken string
}

func (m *mockProviderGateway) AuthURL(state string) string {
	return "https://provider.test/authorize?state=" + state
}

func (m *mockProviderGateway) ExchangeCode(_ context.Context, _ string) (port.ProviderToken, error) {
	return m.exchanged, m.exchangeErr
}

func (m *mockProviderGateway) Refresh(_ context.Context, _ string) (port.ProviderToken, error) {
	m.refreshCalled = true
	return m.refreshed, m.refreshErr
}

func (m *mockProviderGateway) FetchActivities(_ context.Context, accessToken string, _ time.Time) ([]port.ProviderActivity, error) {
	m.usedAccessToken = accessToken
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.activities, nil
}

func (m *mockProviderGateway) Provider() string {
	if m.provider == "" {
		return "strava"
	}
	return m.provider
}

var _ port.ProviderGateway = (*mockProviderGateway)(nil)

// mockProviderTokenRepo é um repositório de tokens em memória; a chave é "userID:provider".
type mockProviderTokenRepo struct {
	tokens     map[string]*port.ProviderToken
	saveCalled int
	saveErr    error
	findErr    error
}

func newMockProviderTokenRepo() *mockProviderTokenRepo {
	return &mockProviderTokenRepo{tokens: make(map[string]*port.ProviderToken)}
}

func (m *mockProviderTokenRepo) Save(_ context.Context, userID string, token port.ProviderToken) error {
	m.saveCalled++
	if m.saveErr != nil {
		return m.saveErr
	}
	t := token
	m.tokens[userID+":"+token.Provider] = &t
	return nil
}

func (m *mockProviderTokenRepo) Find(_ context.Context, userID, provider string) (*port.ProviderToken, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.tokens[userID+":"+provider], nil
}

func (m *mockProviderTokenRepo) FindUserByAthlete(_ context.Context, provider, athleteID string) (string, bool, error) {
	if m.findErr != nil {
		return "", false, m.findErr
	}
	for key, tok := range m.tokens {
		if tok.Provider == provider && tok.AthleteID == athleteID {
			// chave é "userID:provider"; recupera o userID.
			return key[:len(key)-len(provider)-1], true, nil
		}
	}
	return "", false, nil
}

var _ port.ProviderTokenRepository = (*mockProviderTokenRepo)(nil)
