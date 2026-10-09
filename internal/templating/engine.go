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
		logger.Bool("include_runways", opts.IncludeRunways))

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
		data.ActiveRunways = FormatActiveRunwaysData(context.ActiveRunways)
	} else {
		data.Runways = "Runway information not available."
		data.ActiveRunways = "Active runway detection not available."
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
