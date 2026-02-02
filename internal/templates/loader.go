package templates

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/terra-clan/sandbox-engine/internal/models"
)

// Loader manages loading and caching of templates
type Loader struct {
	mu        sync.RWMutex
	templates map[string]*models.Template
}

// NewLoader creates a new template loader
func NewLoader() *Loader {
	return &Loader{
		templates: make(map[string]*models.Template),
	}
}

// LoadFromDir loads all YAML templates from a directory
func (l *Loader) LoadFromDir(dir string) error {
	slog.Info("loading templates from directory", "dir", dir)

	// Find all YAML files
	patterns := []string{"*.yaml", "*.yml"}
	var files []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			continue
		}
		files = append(files, matches...)

		// Also check subdirectories
		subMatches, err := filepath.Glob(filepath.Join(dir, "*", pattern))
		if err != nil {
			continue
		}
		files = append(files, subMatches...)
	}

	if len(files) == 0 {
		slog.Warn("no template files found", "dir", dir)
		return nil
	}

	loaded := 0
	for _, file := range files {
		if err := l.LoadFromFile(file); err != nil {
			slog.Warn("failed to load template", "file", file, "error", err)
			continue
		}
		loaded++
	}

	slog.Info("templates loaded", "count", loaded, "total_files", len(files))
	return nil
}

// LoadFromFile loads a single template from a YAML file
func (l *Loader) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var tmpl templateFile
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate required fields
	if tmpl.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if tmpl.BaseImage == "" {
		return fmt.Errorf("base_image is required")
	}

	// Convert TTL string to duration
	ttl := 1 * time.Hour
	if tmpl.TTL != "" {
		if d, err := time.ParseDuration(tmpl.TTL); err == nil {
			ttl = d
		}
	}

	template := &models.Template{
		Name:        tmpl.Name,
		Description: tmpl.Description,
		BaseImage:   tmpl.BaseImage,
		Services:    tmpl.Services,
		Resources:   tmpl.Resources,
		Env:         tmpl.Env,
		TTL:         ttl,
		Expose:      tmpl.Expose,
		Volumes:     tmpl.Volumes,
		Commands:    tmpl.Commands,
		Labels:      tmpl.Labels,
	}

	// Apply defaults
	if template.Resources.CPULimit == "" {
		template.Resources.CPULimit = "1"
	}
	if template.Resources.MemoryLimit == "" {
		template.Resources.MemoryLimit = "512m"
	}

	l.mu.Lock()
	l.templates[tmpl.Name] = template
	l.mu.Unlock()

	slog.Info("template loaded", "name", tmpl.Name, "image", tmpl.BaseImage)
	return nil
}

// Get retrieves a template by name
func (l *Loader) Get(name string) *models.Template {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.templates[name]
}

// List returns all loaded templates
func (l *Loader) List() []*models.Template {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*models.Template, 0, len(l.templates))
	for _, tmpl := range l.templates {
		result = append(result, tmpl)
	}
	return result
}

// Add programmatically adds a template
func (l *Loader) Add(template *models.Template) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.templates[template.Name] = template
}

// Remove removes a template by name
func (l *Loader) Remove(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.templates, name)
}

// templateFile represents the YAML structure of a template file
type templateFile struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	BaseImage   string            `yaml:"base_image"`
	Services    []string          `yaml:"services"`
	Resources   models.Resources  `yaml:"resources"`
	Env         map[string]string `yaml:"env"`
	TTL         string            `yaml:"ttl"`
	Expose      []models.Port     `yaml:"expose"`
	Volumes     []models.Volume   `yaml:"volumes"`
	Commands    models.Commands   `yaml:"commands"`
	Labels      map[string]string `yaml:"labels"`
}
