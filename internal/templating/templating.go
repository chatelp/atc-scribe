package templating

import (
	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/weather"
	"github.com/yegors/co-atc/pkg/logger"
)

// Service provides the main templating functionality
type Service struct {
	engine     *Engine
	aggregator *DataAggregator
	logger     *logger.Logger
}

// NewService creates a new templating service
func NewService(
	adsbService *adsb.Service,
	weatherService *weather.Service,
	config *config.Config,
	logger *logger.Logger,
) *Service {
	// Create data aggregator
	aggregator := NewDataAggregator(
		adsbService,
		weatherService,
		config,
		logger,
	)

	// Create template engine
	engine := NewEngine(aggregator, logger)

	return &Service{
		engine:     engine,
		aggregator: aggregator,
		logger:     logger.Named("templating-service"),
	}
}

// RenderPostProcessorTemplate renders the post-processor template without transcription history
func (s *Service) RenderPostProcessorTemplate(templatePath string) (string, error) {
	opts := PostProcessorFormattingOptions()
	return s.engine.RenderTemplate(templatePath, opts)
}
