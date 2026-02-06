package templates

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/terra-clan/sandbox-engine/internal/models"
)

// Loader manages loading and caching of templates and catalog
type Loader struct {
	mu        sync.RWMutex
	templates map[string]*models.Template

	// Catalog data (populated from hierarchical directory structure)
	domains  map[string]*models.Domain
	projects map[string]*models.CatalogProject
	tasks    map[string]*models.CatalogTask
}

// NewLoader creates a new template loader
func NewLoader() *Loader {
	return &Loader{
		templates: make(map[string]*models.Template),
		domains:   make(map[string]*models.Domain),
		projects:  make(map[string]*models.CatalogProject),
		tasks:     make(map[string]*models.CatalogTask),
	}
}

// LoadFromDir loads all YAML templates from a directory (flat and hierarchical)
func (l *Loader) LoadFromDir(dir string) error {
	slog.Info("loading templates from directory", "dir", dir)

	// Find all YAML files (flat loading for backward compat)
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

	loaded := 0
	for _, file := range files {
		// Skip catalog-specific files (parsed in loadCatalogFromDir)
		base := filepath.Base(file)
		if base == "domain.yaml" || base == "domain.yml" {
			continue
		}

		if err := l.LoadFromFile(file); err != nil {
			slog.Warn("failed to load template", "file", file, "error", err)
			continue
		}
		loaded++
	}

	slog.Info("templates loaded (flat)", "count", loaded, "total_files", len(files))

	// Load hierarchical catalog (domain → project → task)
	if err := l.loadCatalogFromDir(dir); err != nil {
		slog.Warn("failed to load catalog", "error", err)
	}

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

// --- Catalog accessors ---

// ListDomains returns all loaded domains
func (l *Loader) ListDomains() []*models.Domain {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*models.Domain, 0, len(l.domains))
	for _, d := range l.domains {
		result = append(result, d)
	}
	return result
}

// GetDomain returns a domain by ID
func (l *Loader) GetDomain(id string) *models.Domain {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.domains[id]
}

// ListProjects returns all projects for a given domain
func (l *Loader) ListProjects(domainID string) []*models.CatalogProject {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*models.CatalogProject
	for _, p := range l.projects {
		if p.DomainID == domainID {
			result = append(result, p)
		}
	}
	return result
}

// GetProject returns a project by ID (e.g. "fintech/python-trading")
func (l *Loader) GetProject(id string) *models.CatalogProject {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.projects[id]
}

// ListTasks returns all tasks for a given project
func (l *Loader) ListTasks(projectID string) []*models.CatalogTask {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*models.CatalogTask
	for _, t := range l.tasks {
		if t.ProjectID == projectID {
			result = append(result, t)
		}
	}
	return result
}

// GetTask returns a task by ID (e.g. "fintech/python-trading/limit-orders")
func (l *Loader) GetTask(id string) *models.CatalogTask {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tasks[id]
}

// --- Catalog loading ---

// loadCatalogFromDir scans for domain.yaml directories and builds the catalog hierarchy
func (l *Loader) loadCatalogFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		domainDir := filepath.Join(dir, entry.Name())
		domainYaml := filepath.Join(domainDir, "domain.yaml")

		if _, err := os.Stat(domainYaml); os.IsNotExist(err) {
			continue // not a domain directory
		}

		domain, err := l.loadDomain(entry.Name(), domainDir)
		if err != nil {
			slog.Warn("failed to load domain", "dir", entry.Name(), "error", err)
			continue
		}

		l.mu.Lock()
		l.domains[domain.ID] = domain
		l.mu.Unlock()

		slog.Info("catalog domain loaded", "id", domain.ID, "name", domain.Name,
			"projects", domain.ProjectsCount, "tasks", domain.TasksCount)
	}

	return nil
}

// loadDomain loads a single domain and its projects/tasks
func (l *Loader) loadDomain(id string, dir string) (*models.Domain, error) {
	// Parse domain.yaml
	data, err := os.ReadFile(filepath.Join(dir, "domain.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to read domain.yaml: %w", err)
	}

	var df domainFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		return nil, fmt.Errorf("failed to parse domain.yaml: %w", err)
	}

	domain := &models.Domain{
		ID:          id,
		Name:        df.Name,
		Description: df.Description,
	}

	// Scan for project subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read domain dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(dir, entry.Name())
		templateYaml := filepath.Join(projectDir, "template.yaml")

		if _, err := os.Stat(templateYaml); os.IsNotExist(err) {
			continue // not a project directory
		}

		project, err := l.loadProject(id, entry.Name(), projectDir)
		if err != nil {
			slog.Warn("failed to load project", "domain", id, "project", entry.Name(), "error", err)
			continue
		}

		l.mu.Lock()
		l.projects[project.ID] = project
		l.mu.Unlock()

		domain.ProjectsCount++
		domain.TasksCount += project.TasksCount
	}

	return domain, nil
}

// loadProject loads a project (template + tasks) within a domain
func (l *Loader) loadProject(domainID, projectName, dir string) (*models.CatalogProject, error) {
	templatePath := filepath.Join(dir, "template.yaml")

	// Load the template using existing mechanism (registers under YAML name)
	if err := l.LoadFromFile(templatePath); err != nil {
		return nil, fmt.Errorf("failed to load template: %w", err)
	}

	projectID := domainID + "/" + projectName

	// Read the YAML name to find the template in cache
	data, _ := os.ReadFile(templatePath)
	var tf templateFile
	yaml.Unmarshal(data, &tf)

	// Get the loaded template and register alias under projectID
	name := projectName
	description := ""

	l.mu.Lock()
	if tf.Name != "" {
		if tmpl, ok := l.templates[tf.Name]; ok {
			name = tmpl.Name
			description = tmpl.Description
			// Register alias so template is also accessible by projectID
			l.templates[projectID] = tmpl
		}
	}
	l.mu.Unlock()

	project := &models.CatalogProject{
		ID:          projectID,
		DomainID:    domainID,
		Name:        name,
		Description: description,
	}

	// Load tasks
	tasksDir := filepath.Join(dir, "tasks")
	if _, err := os.Stat(tasksDir); err == nil {
		tasks, err := l.loadTasks(domainID, projectID, tasksDir)
		if err != nil {
			slog.Warn("failed to load tasks", "project", projectID, "error", err)
		} else {
			project.TasksCount = len(tasks)
		}
	}

	return project, nil
}

// loadTasks loads all task YAML files from a tasks/ directory
func (l *Loader) loadTasks(domainID, projectID, dir string) ([]*models.CatalogTask, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks dir: %w", err)
	}

	var tasks []*models.CatalogTask

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		taskPath := filepath.Join(dir, entry.Name())
		task, err := l.loadTask(domainID, projectID, taskPath)
		if err != nil {
			slog.Warn("failed to load task", "file", entry.Name(), "error", err)
			continue
		}

		l.mu.Lock()
		l.tasks[task.ID] = task
		l.mu.Unlock()

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// loadTask loads a single task YAML file
func (l *Loader) loadTask(domainID, projectID, path string) (*models.CatalogTask, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read task file: %w", err)
	}

	var tf taskFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("failed to parse task YAML: %w", err)
	}

	// Use code from YAML, fall back to filename without extension
	code := tf.Code
	if code == "" {
		base := filepath.Base(path)
		code = strings.TrimSuffix(base, filepath.Ext(base))
	}

	if tf.Title == "" {
		return nil, fmt.Errorf("task title is required")
	}

	taskID := projectID + "/" + code

	var requiredLevel *string
	if tf.RequiredLevel != "" {
		requiredLevel = &tf.RequiredLevel
	}

	return &models.CatalogTask{
		ID:            taskID,
		Code:          code,
		Title:         tf.Title,
		Description:   tf.Description,
		Difficulty:    tf.Difficulty,
		RequiredLevel: requiredLevel,
		TimeLimit:     tf.TimeLimit,
		Skills:        tf.Skills,
		DomainID:      domainID,
		ProjectID:     projectID,
	}, nil
}

// --- YAML file structs ---

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

// domainFile represents the YAML structure of a domain.yaml file
type domainFile struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// taskFile represents the YAML structure of a task YAML file
type taskFile struct {
	Code          string   `yaml:"code"`
	Title         string   `yaml:"title"`
	Description   string   `yaml:"description"`
	Difficulty    string   `yaml:"difficulty"`
	RequiredLevel string   `yaml:"required_level"`
	TimeLimit     int      `yaml:"time_limit"`
	Skills        []string `yaml:"skills"`
}
