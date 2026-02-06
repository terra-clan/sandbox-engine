package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCatalogFromDir(t *testing.T) {
	// Use the actual templates directory
	templatesDir := filepath.Join("..", "..", "templates")

	// Check it exists
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		t.Skip("templates directory not found, skipping")
	}

	loader := NewLoader()
	if err := loader.LoadFromDir(templatesDir); err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	// Check domains loaded
	domains := loader.ListDomains()
	if len(domains) < 2 {
		t.Errorf("expected at least 2 domains, got %d", len(domains))
	}

	// Check fintech domain
	fintech := loader.GetDomain("fintech")
	if fintech == nil {
		t.Fatal("fintech domain not found")
	}
	if fintech.Name != "Финтех" {
		t.Errorf("expected fintech name 'Финтех', got '%s'", fintech.Name)
	}
	if fintech.ProjectsCount < 2 {
		t.Errorf("expected at least 2 projects in fintech, got %d", fintech.ProjectsCount)
	}
	if fintech.TasksCount < 4 {
		t.Errorf("expected at least 4 tasks in fintech, got %d", fintech.TasksCount)
	}

	// Check ecommerce domain
	ecommerce := loader.GetDomain("ecommerce")
	if ecommerce == nil {
		t.Fatal("ecommerce domain not found")
	}
	if ecommerce.ProjectsCount < 2 {
		t.Errorf("expected at least 2 projects in ecommerce, got %d", ecommerce.ProjectsCount)
	}

	// Check projects
	fintechProjects := loader.ListProjects("fintech")
	if len(fintechProjects) < 2 {
		t.Errorf("expected at least 2 fintech projects, got %d", len(fintechProjects))
	}

	pythonTrading := loader.GetProject("fintech/python-trading")
	if pythonTrading == nil {
		t.Fatal("fintech/python-trading project not found")
	}
	if pythonTrading.TasksCount < 2 {
		t.Errorf("expected at least 2 tasks in python-trading, got %d", pythonTrading.TasksCount)
	}

	// Check tasks
	tasks := loader.ListTasks("fintech/python-trading")
	if len(tasks) < 2 {
		t.Errorf("expected at least 2 tasks, got %d", len(tasks))
	}

	limitOrders := loader.GetTask("fintech/python-trading/limit-orders")
	if limitOrders == nil {
		t.Fatal("limit-orders task not found")
	}
	if limitOrders.Title != "Реализация лимитных ордеров" {
		t.Errorf("unexpected task title: %s", limitOrders.Title)
	}
	if limitOrders.Difficulty != "hard" {
		t.Errorf("expected difficulty 'hard', got '%s'", limitOrders.Difficulty)
	}
	if limitOrders.RequiredLevel == nil || *limitOrders.RequiredLevel != "senior" {
		t.Error("expected requiredLevel 'senior'")
	}
	if limitOrders.TimeLimit != 7200 {
		t.Errorf("expected timeLimit 7200, got %d", limitOrders.TimeLimit)
	}

	// Check template backward compatibility — original YAML name still works
	tmpl := loader.Get("fintech-python")
	if tmpl == nil {
		t.Fatal("template 'fintech-python' not accessible by original name")
	}

	// Check template accessible by projectID too
	tmplByProject := loader.Get("fintech/python-trading")
	if tmplByProject == nil {
		t.Fatal("template not accessible by project ID 'fintech/python-trading'")
	}
	if tmpl != tmplByProject {
		t.Error("template by name and by projectID should be the same pointer")
	}

	// Log summary
	t.Logf("Domains: %d", len(domains))
	for _, d := range domains {
		t.Logf("  %s (%s): %d projects, %d tasks", d.ID, d.Name, d.ProjectsCount, d.TasksCount)
	}
}
