package handler_test

import (
	"context"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type handlerMocks struct {
	registerActivity *usecase.RegisterActivity
	recordMetric     *usecase.RecordMetric
	queryMetrics     *usecase.QueryMetrics
	generateInsights *usecase.GenerateInsights
}

func newHandlerMocks() handlerMocks {
	actRepo := &inMemoryActivityRepo{
		activities: make(map[string]*entity.Activity),
		byExternal: make(map[string]*entity.Activity),
	}
	metRepo := &inMemoryMetricRepo{}
	insRepo := &inMemoryInsightRepo{}
	pub := &noopPublisher{}
	eval := &noopEvaluator{}

	return handlerMocks{
		registerActivity: usecase.NewRegisterActivity(actRepo, pub),
		recordMetric:     usecase.NewRecordMetric(metRepo, pub),
		queryMetrics:     usecase.NewQueryMetrics(metRepo),
		generateInsights: usecase.NewGenerateInsights(metRepo, insRepo, eval),
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
