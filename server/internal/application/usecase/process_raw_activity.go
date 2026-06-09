package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

// ErrInvalidRawActivity sinaliza um payload raw.activity.imported inválido. É
// um erro de validação: a mensagem é "envenenada" e não melhora com requeue,
// por isso o handler deve nack-á-la para a dead-letter sem retry infinito.
var ErrInvalidRawActivity = errors.New("invalid raw activity payload")

// InsightGeneratedEventType é o Type publicado para cada insight recém-gerado
// pelo pipeline de processamento. O módulo de Notificações o consome.
const InsightGeneratedEventType = "insight.generated"

// InsightGenerated é o payload do evento insight.generated. Carrega o mínimo
// necessário para a notificação sem acoplar ao agregado de domínio.
type InsightGenerated struct {
	UserID    string    `json:"user_id"`
	InsightID string    `json:"insight_id"`
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Date      time.Time `json:"date"`
}

// registerActivityUseCase é o seam para reusar RegisterActivity (persistência +
// deduplicação). *RegisterActivity o satisfaz.
type registerActivityUseCase interface {
	Execute(ctx context.Context, input RegisterActivityInput) (*RegisterActivityOutput, error)
}

// generateInsightsUseCase é o seam para reusar GenerateInsights.
// *GenerateInsights o satisfaz.
type generateInsightsUseCase interface {
	Execute(ctx context.Context, input GenerateInsightsInput) (*GenerateInsightsOutput, error)
}

// aggregateDailyMetricsUseCase é o seam para reusar AggregateDailyMetrics.
// *AggregateDailyMetrics o satisfaz.
type aggregateDailyMetricsUseCase interface {
	Execute(ctx context.Context, input AggregateDailyMetricsInput) (*AggregateDailyMetricsOutput, error)
}

// Compile-time: as use cases concretas reusadas satisfazem os seams injetados.
var (
	_ registerActivityUseCase      = (*RegisterActivity)(nil)
	_ generateInsightsUseCase      = (*GenerateInsights)(nil)
	_ aggregateDailyMetricsUseCase = (*AggregateDailyMetrics)(nil)
)

// ProcessRawActivity é a use case central do módulo de Processamento. Roda o
// pipeline para uma atividade raw importada de um provedor:
// validação -> normalização -> persistência+dedup -> agregação -> insights ->
// publicação de eventos derivados (insight.generated).
//
// Reusa RegisterActivity (que já deduplica por external_id) e GenerateInsights
// em vez de reimplementar a lógica.
type ProcessRawActivity struct {
	register  registerActivityUseCase
	insights  generateInsightsUseCase
	aggregate aggregateDailyMetricsUseCase
	publisher port.EventPublisher
}

func NewProcessRawActivity(
	register registerActivityUseCase,
	insights generateInsightsUseCase,
	aggregate aggregateDailyMetricsUseCase,
	publisher port.EventPublisher,
) *ProcessRawActivity {
	return &ProcessRawActivity{
		register:  register,
		insights:  insights,
		aggregate: aggregate,
		publisher: publisher,
	}
}

func (uc *ProcessRawActivity) Execute(ctx context.Context, raw RawActivityImported) error {
	// Mede a duração total do pipeline de processamento (latência do worker).
	start := time.Now()
	defer func() {
		port.MetricsFromContext(ctx).RecordDuration(ctx, "gah.raw_activity.process.duration", time.Since(start))
	}()

	// 1. Validação.
	if err := validateRawActivity(raw); err != nil {
		return err
	}

	// 2. Normalização para a entrada canônica do GAH.
	input := RegisterActivityInput{
		UserID:       raw.UserID,
		ActivityType: string(normalizeActivityType(raw.Type)),
		Date:         raw.StartTime,
		Duration:     time.Duration(raw.DurationNs),
		AvgHeartRate: raw.AvgHeartRate,
		ExternalID:   raw.ExternalID,
	}

	// 3. Persistência + deduplicação (RegisterActivity já deduplica por
	// external_id). Duplicado é sucesso idempotente: a mensagem já foi
	// processada antes, então ack sem reprocessar insights.
	if _, err := uc.register.Execute(ctx, input); err != nil {
		if errors.Is(err, ErrDuplicateActivity) {
			port.LoggerFromContext(ctx).Info(ctx, "process_raw_activity: duplicate external_id (idempotent ack)",
				"user_id", raw.UserID, "external_id", raw.ExternalID)
			return nil
		}
		return fmt.Errorf("register activity: %w", err)
	}

	// 4. Agregação diária (idempotente via upsert). Falha aqui propaga para
	// nack -> a mensagem é reprocessada/dead-lettered.
	if _, err := uc.aggregate.Execute(ctx, AggregateDailyMetricsInput{UserID: raw.UserID, Day: raw.StartTime}); err != nil {
		return fmt.Errorf("aggregate daily metrics: %w", err)
	}

	// 5. Geração de insights a partir das métricas do usuário.
	out, err := uc.insights.Execute(ctx, GenerateInsightsInput{UserID: raw.UserID})
	if err != nil {
		return fmt.Errorf("generate insights: %w", err)
	}

	// 6. Publica um insight.generated por insight recém-gerado.
	if len(out.Insights) > 0 {
		port.MetricsFromContext(ctx).IncCounter(ctx, "gah.insights.generated", int64(len(out.Insights)))
	}
	for _, ins := range out.Insights {
		event := port.Event{
			Type: InsightGeneratedEventType,
			Payload: InsightGenerated{
				UserID:    ins.UserID,
				InsightID: ins.ID,
				Type:      string(ins.Type),
				Severity:  string(ins.Severity),
				Message:   ins.Message,
				Date:      ins.Date,
			},
		}
		if err := uc.publisher.Publish(ctx, event); err != nil {
			// Não falha o pipeline por falha de publicação: a atividade já foi
			// persistida (efeito principal). Apenas loga — reprocessar geraria
			// duplicatas de insights.
			port.LoggerFromContext(ctx).Error(ctx, "process_raw_activity: publish insight.generated failed",
				"event", InsightGeneratedEventType, "user_id", ins.UserID, "insight_id", ins.ID, "error", err)
		}
	}

	return nil
}

// validateRawActivity rejeita payloads claramente inválidos antes de qualquer
// efeito colateral. Retorna ErrInvalidRawActivity envolvendo o motivo.
func validateRawActivity(raw RawActivityImported) error {
	switch {
	case strings.TrimSpace(raw.UserID) == "":
		return fmt.Errorf("%w: empty user_id", ErrInvalidRawActivity)
	case strings.TrimSpace(raw.ExternalID) == "":
		return fmt.Errorf("%w: empty external_id", ErrInvalidRawActivity)
	case raw.DurationNs <= 0:
		return fmt.Errorf("%w: non-positive duration", ErrInvalidRawActivity)
	case raw.StartTime.IsZero():
		return fmt.Errorf("%w: zero start_time", ErrInvalidRawActivity)
	case raw.StartTime.After(time.Now().Add(time.Hour)):
		// Tolerância de 1h para clock skew; além disso é claramente inválido.
		return fmt.Errorf("%w: start_time in the future", ErrInvalidRawActivity)
	case raw.AvgHeartRate != 0 && (raw.AvgHeartRate < 30 || raw.AvgHeartRate > 220):
		return fmt.Errorf("%w: avg_heart_rate out of range", ErrInvalidRawActivity)
	}
	return nil
}

// activityTypeAliases mapeia tipos canônicos de provedores (ex.: nomes da
// Strava) para os ActivityType do GAH. O lookup é case-insensitive.
var activityTypeAliases = map[string]valueobject.ActivityType{
	"run":            valueobject.ActivityTypeRunning,
	"running":        valueobject.ActivityTypeRunning,
	"trailrun":       valueobject.ActivityTypeRunning,
	"virtualrun":     valueobject.ActivityTypeRunning,
	"ride":           valueobject.ActivityTypeCycling,
	"cycling":        valueobject.ActivityTypeCycling,
	"virtualride":    valueobject.ActivityTypeCycling,
	"ebikeride":      valueobject.ActivityTypeCycling,
	"swim":           valueobject.ActivityTypeSwimming,
	"swimming":       valueobject.ActivityTypeSwimming,
	"weighttraining": valueobject.ActivityTypeWeightlifting,
	"weightlifting":  valueobject.ActivityTypeWeightlifting,
	"workout":        valueobject.ActivityTypeWeightlifting,
	"yoga":           valueobject.ActivityTypeYoga,
	"hike":           valueobject.ActivityTypeHiking,
	"hiking":         valueobject.ActivityTypeHiking,
	"crossfit":       valueobject.ActivityTypeCrossfit,
}

// normalizeActivityType mapeia o tipo canônico do provedor para um ActivityType
// do GAH; tipos desconhecidos viram "other" (nunca falha a normalização).
func normalizeActivityType(raw string) valueobject.ActivityType {
	key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(raw), " ", ""))
	if at, ok := activityTypeAliases[key]; ok {
		return at
	}
	if at := valueobject.ActivityType(key); at.IsValid() {
		return at
	}
	return valueobject.ActivityTypeOther
}
