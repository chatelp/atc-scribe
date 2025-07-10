package templating

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sync"
	"text/template"

	"github.com/yegors/co-atc/pkg/logger"
)

// Engine handles template loading, caching, and rendering
type Engine struct {
	aggregator    *DataAggregator
	templateCache map[string]*template.Template
	cacheMutex    sync.RWMutex
	logger        *logger.Logger
}

// NewEngine creates a new template engine
func NewEngine(aggregator *DataAggregator, logger *logger.Logger) *Engine {
	return &Engine{
		aggregator:    aggregator,
		templateCache: make(map[string]*template.Template),
		logger:        logger.Named("template-engine"),
	}
}

// RenderTemplate renders a template with current airspace data
func (e *Engine) RenderTemplate(templatePath string, opts FormattingOptions) (string, error) {
	e.logger.Debug("Rendering template",
		logger.String("template_path", templatePath),
		logger.Int("max_aircraft", opts.MaxAircraft),
		logger.Bool("include_weather", opts.IncludeWeather),
		logger.Bool("include_runways", opts.IncludeRunways),
		logger.Bool("include_transcription_history", opts.IncludeTranscriptionHistory))

	// Load template if not in cache
	tmpl, err := e.getTemplate(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to get template: %w", err)
	}

	// Get template context from aggregator
	context, err := e.aggregator.GetTemplateContext(opts)
	if err != nil {
		return "", fmt.Errorf("failed to get template context: %w", err)
	}

	// Format the data for template rendering
	data := e.prepareTemplateData(context, opts)

	// Render the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	rendered := buf.String()
	e.logger.Debug("Template rendered successfully",
		logger.String("template_path", templatePath),
		logger.Int("rendered_length", len(rendered)))

	return rendered, nil
}

// RenderTemplateWithContext renders a template with pre-aggregated context data
func (e *Engine) RenderTemplateWithContext(templatePath string, context *TemplateContext, opts FormattingOptions) (string, error) {
	e.logger.Debug("Rendering template with provided context",
		logger.String("template_path", templatePath))

	// Load template if not in cache
	tmpl, err := e.getTemplate(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to get template: %w", err)
	}

	// Format the data for template rendering
	data := e.prepareTemplateData(context, opts)

	// Render the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	rendered := buf.String()
	e.logger.Debug("Template rendered successfully with context",
		logger.String("template_path", templatePath),
		logger.Int("rendered_length", len(rendered)))

	return rendered, nil
}

// prepareTemplateData converts raw context data to formatted template data
func (e *Engine) prepareTemplateData(context *TemplateContext, opts FormattingOptions) TemplateData {
	data := TemplateData{
		Timestamp: context.Timestamp,
		Time:      context.Timestamp.Format(opts.TimeFormat),
	}

	// Format aircraft data
	data.Aircraft = FormatAircraftData(context.Aircraft, context.Airport)

	// Format weather data if available
	if opts.IncludeWeather && context.Weather != nil {
		data.Weather = FormatWeatherData(context.Weather)
	} else {
		data.Weather = "Weather data not available."
	}

	// Format runway data if available
	if opts.IncludeRunways {
		data.Runways = FormatRunwayData(context.Runways)
	} else {
		data.Runways = "Runway information not available."
	}

	// Format transcription history if requested (only for ATC Chat)
	if opts.IncludeTranscriptionHistory {
		data.TranscriptionHistory = FormatTranscriptionHistory(context.TranscriptionHistory)
	} else {
		data.TranscriptionHistory = ""
	}

	// Format airport data
	data.Airport = FormatAirportData(context.Airport)

	return data
}

// getTemplate retrieves a template from cache or loads it from file
func (e *Engine) getTemplate(templatePath string) (*template.Template, error) {
	// Check cache first (read lock)
	e.cacheMutex.RLock()
	if tmpl, exists := e.templateCache[templatePath]; exists {
		e.cacheMutex.RUnlock()
		return tmpl, nil
	}
	e.cacheMutex.RUnlock()

	// Template not in cache, load it (write lock)
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	// Double-check in case another goroutine loaded it while we were waiting
	if tmpl, exists := e.templateCache[templatePath]; exists {
		return tmpl, nil
	}

	// Load template from file
	tmpl, err := e.loadTemplate(templatePath)
	if err != nil {
		return nil, err
	}

	// Cache the template
	e.templateCache[templatePath] = tmpl
	e.logger.Debug("Template loaded and cached",
		logger.String("template_path", templatePath))

	return tmpl, nil
}

// loadTemplate loads a template from file
func (e *Engine) loadTemplate(templatePath string) (*template.Template, error) {
	content, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file '%s': %w", templatePath, err)
	}

	tmpl, err := template.New(templatePath).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template file '%s': %w", templatePath, err)
	}

	return tmpl, nil
}

// ReloadTemplate forces a template to be reloaded from file
func (e *Engine) ReloadTemplate(templatePath string) error {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	// Load template from file
	tmpl, err := e.loadTemplate(templatePath)
	if err != nil {
		return err
	}

	// Update cache
	e.templateCache[templatePath] = tmpl
	e.logger.Info("Template reloaded",
		logger.String("template_path", templatePath))

	return nil
}

// ReloadAllTemplates forces all cached templates to be reloaded from files
func (e *Engine) ReloadAllTemplates() error {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	var errors []string
	reloadedCount := 0

	for templatePath := range e.templateCache {
		tmpl, err := e.loadTemplate(templatePath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", templatePath, err))
			continue
		}
		e.templateCache[templatePath] = tmpl
		reloadedCount++
	}

	if len(errors) > 0 {
		e.logger.Error("Some templates failed to reload",
			logger.Int("successful", reloadedCount),
			logger.Int("failed", len(errors)))
		return fmt.Errorf("failed to reload %d templates: %v", len(errors), errors)
	}

	e.logger.Info("All templates reloaded successfully",
		logger.Int("count", reloadedCount))

	return nil
}

// ClearCache clears the template cache
func (e *Engine) ClearCache() {
	e.cacheMutex.Lock()
	defer e.cacheMutex.Unlock()

	templateCount := len(e.templateCache)
	e.templateCache = make(map[string]*template.Template)

	e.logger.Info("Template cache cleared",
		logger.Int("cleared_count", templateCount))
}

// GetCacheStats returns statistics about the template cache
func (e *Engine) GetCacheStats() map[string]interface{} {
	e.cacheMutex.RLock()
	defer e.cacheMutex.RUnlock()

	templates := make([]string, 0, len(e.templateCache))
	for path := range e.templateCache {
		templates = append(templates, path)
	}

	return map[string]interface{}{
		"cached_template_count": len(e.templateCache),
		"cached_templates":      templates,
	}
}

// GetRawTemplate returns the raw template content without processing
func (e *Engine) GetRawTemplate(templatePath string) (string, error) {
	content, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file '%s': %w", templatePath, err)
	}
	return string(content), nil
}
