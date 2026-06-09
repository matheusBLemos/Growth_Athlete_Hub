package usecase_test

import (
	"context"
	"errors"
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
