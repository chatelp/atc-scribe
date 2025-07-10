package templating

import (
	"github.com/yegors/co-atc/internal/adsb"
	"github.com/yegors/co-atc/internal/config"
	"github.com/yegors/co-atc/internal/frequencies"
	"github.com/yegors/co-atc/internal/storage/sqlite"
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
	transcriptionStorage *sqlite.TranscriptionStorage,
	frequencyService *frequencies.Service,
	config *config.Config,
	logger *logger.Logger,
) *Service {
	// Create data aggregator
	aggregator := NewDataAggregator(
		adsbService,
		weatherService,
		transcriptionStorage,
		frequencyService,
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

// RenderATCChatTemplate renders the ATC chat template with full context
func (s *Service) RenderATCChatTemplate(templatePath string) (string, error) {
	opts := ATCChatFormattingOptions()
	return s.engine.RenderTemplate(templatePath, opts)
}

// RenderPostProcessorTemplate renders the post-processor template without transcription history
func (s *Service) RenderPostProcessorTemplate(templatePath string) (string, error) {
	opts := PostProcessorFormattingOptions()
	return s.engine.RenderTemplate(templatePath, opts)
}

// RenderTemplate renders a template with custom formatting options
func (s *Service) RenderTemplate(templatePath string, opts FormattingOptions) (string, error) {
	return s.engine.RenderTemplate(templatePath, opts)
}

// GetTemplateContext gets the current airspace context for custom processing
func (s *Service) GetTemplateContext(opts FormattingOptions) (*TemplateContext, error) {
	return s.aggregator.GetTemplateContext(opts)
}

// RenderTemplateWithContext renders a template with pre-aggregated context
func (s *Service) RenderTemplateWithContext(templatePath string, context *TemplateContext, opts FormattingOptions) (string, error) {
	return s.engine.RenderTemplateWithContext(templatePath, context, opts)
}

// ReloadTemplate forces a template to be reloaded from file
func (s *Service) ReloadTemplate(templatePath string) error {
	return s.engine.ReloadTemplate(templatePath)
}

// ReloadAllTemplates forces all cached templates to be reloaded
func (s *Service) ReloadAllTemplates() error {
	return s.engine.ReloadAllTemplates()
}

// ClearCache clears the template cache
func (s *Service) ClearCache() {
	s.engine.ClearCache()
}

// GetCacheStats returns statistics about the template cache
func (s *Service) GetCacheStats() map[string]interface{} {
	return s.engine.GetCacheStats()
}

// GetRawTemplate returns the raw template content without processing
func (s *Service) GetRawTemplate(templatePath string) (string, error) {
	return s.engine.GetRawTemplate(templatePath)
}
