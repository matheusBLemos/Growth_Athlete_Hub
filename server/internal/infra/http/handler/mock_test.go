package handler_test

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/http/middleware"
)

type handlerMocks struct {
	registerActivity *usecase.RegisterActivity
	recordMetric     *usecase.RecordMetric
	queryMetrics     *usecase.QueryMetrics
	generateInsights *usecase.GenerateInsights
	registerUser     *usecase.RegisterUser
	loginUser        *usecase.LoginUser
}

func newHandlerMocks() handlerMocks {
	actRepo := &inMemoryActivityRepo{
		activities: make(map[string]*entity.Activity),
		byExternal: make(map[string]*entity.Activity),
	}
	metRepo := &inMemoryMetricRepo{}
	insRepo := &inMemoryInsightRepo{}
	userRepo := &inMemoryUserRepo{
		byID:    make(map[string]*entity.User),
		byEmail: make(map[string]*entity.User),
	}
	pub := &noopPublisher{}
	eval := &noopEvaluator{}
	hasher := &fakeHasher{}
	issuer := &fakeIssuer{}

	return handlerMocks{
		registerActivity: usecase.NewRegisterActivity(actRepo, pub),
		recordMetric:     usecase.NewRecordMetric(metRepo, pub, nil, 0),
		queryMetrics:     usecase.NewQueryMetrics(metRepo, nil, 0),
		generateInsights: usecase.NewGenerateInsights(metRepo, insRepo, eval),
		registerUser:     usecase.NewRegisterUser(userRepo, hasher, pub),
		loginUser:        usecase.NewLoginUser(userRepo, hasher, issuer),
	}
}

// --- In-memory user repo + fake auth deps for handler tests ---

type inMemoryUserRepo struct {
	byID    map[string]*entity.User
	byEmail map[string]*entity.User
}

func (r *inMemoryUserRepo) Save(_ context.Context, u *entity.User) error {
	r.byID[u.ID] = u
	r.byEmail[u.Email] = u
	return nil
}

func (r *inMemoryUserRepo) FindByID(_ context.Context, id string) (*entity.User, error) {
	return r.byID[id], nil
}

func (r *inMemoryUserRepo) FindByEmail(_ context.Context, email string) (*entity.User, error) {
	return r.byEmail[email], nil
}

var _ port.UserRepository = (*inMemoryUserRepo)(nil)

type fakeHasher struct{}

func (fakeHasher) Hash(plain string) (string, error) { return "hashed:" + plain, nil }

func (fakeHasher) Compare(hash, plain string) error {
	if hash != "hashed:"+plain {
		return errInvalid
	}
	return nil
}

var _ port.PasswordHasher = (*fakeHasher)(nil)

type fakeIssuer struct{}

func (fakeIssuer) Issue(userID string) (string, error) { return "token:" + userID, nil }

func (fakeIssuer) Parse(token string) (string, error) {
	const prefix = "token:"
	if len(token) <= len(prefix) || token[:len(prefix)] != prefix {
		return "", errInvalid
	}
	return token[len(prefix):], nil
}

var _ port.TokenIssuer = (*fakeIssuer)(nil)

var errInvalid = errors.New("invalid")

// withUser é um middleware de teste que injeta um userID autenticado em c.Locals,
// simulando o middleware de auth real.
func withUser(userID string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals(middleware.LocalsUserID, userID)
		return c.Next()
	}
}

// --- In-memory repos for handler tests ---

type inMemoryActivityRepo struct {
	activities map[string]*entity.Activity
	byExternal map[string]*entity.Activity
}

func (r *inMemoryActivityRepo) Save(_ context.Context, a *entity.Activity) error {
	r.activities[a.ID] = a
	if a.ExternalID != "" {
		r.byExternal[a.ExternalID] = a
	}
	return nil
}

func (r *inMemoryActivityRepo) FindByID(_ context.Context, id string) (*entity.Activity, error) {
	return r.activities[id], nil
}

func (r *inMemoryActivityRepo) FindByUserID(_ context.Context, _ string, _, _ time.Time) ([]*entity.Activity, error) {
	return nil, nil
}

func (r *inMemoryActivityRepo) FindByExternalID(_ context.Context, eid string) (*entity.Activity, error) {
	return r.byExternal[eid], nil
}

var _ port.ActivityRepository = (*inMemoryActivityRepo)(nil)

type inMemoryMetricRepo struct {
	metrics []*entity.Metric
}

func (r *inMemoryMetricRepo) Save(_ context.Context, m *entity.Metric) error {
	r.metrics = append(r.metrics, m)
	return nil
}

func (r *inMemoryMetricRepo) FindByUserIDAndType(_ context.Context, userID string, mt valueobject.MetricType, from, to time.Time) ([]*entity.Metric, error) {
	var result []*entity.Metric
	for _, m := range r.metrics {
		if m.UserID == userID && m.Type == mt && !m.Date.Before(from) && !m.Date.After(to) {
			result = append(result, m)
		}
	}
	return result, nil
}

func (r *inMemoryMetricRepo) FindLatestByUserIDAndType(_ context.Context, _ string, _ valueobject.MetricType, _ int) ([]*entity.Metric, error) {
	return nil, nil
}

var _ port.MetricRepository = (*inMemoryMetricRepo)(nil)

type inMemoryInsightRepo struct {
	insights []*entity.Insight
}

func (r *inMemoryInsightRepo) Save(_ context.Context, i *entity.Insight) error {
	r.insights = append(r.insights, i)
	return nil
}

func (r *inMemoryInsightRepo) SaveAll(_ context.Context, insights []*entity.Insight) error {
	r.insights = append(r.insights, insights...)
	return nil
}

func (r *inMemoryInsightRepo) FindByUserID(_ context.Context, _ string, _, _ time.Time) ([]*entity.Insight, error) {
	return nil, nil
}

var _ port.InsightRepository = (*inMemoryInsightRepo)(nil)

type noopPublisher struct{}

func (p *noopPublisher) Publish(_ context.Context, _ port.Event) error { return nil }

var _ port.EventPublisher = (*noopPublisher)(nil)

type noopEvaluator struct{}

func (e *noopEvaluator) Evaluate(_ context.Context, _ string, _ []*entity.Metric) ([]*entity.Insight, error) {
	return nil, nil
}

var _ port.InsightEvaluator = (*noopEvaluator)(nil)
