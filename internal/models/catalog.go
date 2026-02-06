package models

// Domain represents a top-level assessment category (e.g., fintech, ecommerce)
type Domain struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	ProjectsCount int    `json:"projectsCount"`
	TasksCount    int    `json:"tasksCount"`
}

// CatalogProject represents a project within a domain (= environment template + tasks)
type CatalogProject struct {
	ID          string `json:"id"`       // "fintech/python-trading"
	DomainID    string `json:"domainId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TasksCount  int    `json:"tasksCount"`
}

// CatalogTask represents a coding task within a project
type CatalogTask struct {
	ID            string   `json:"id"`            // "fintech/python-trading/limit-orders"
	Code          string   `json:"code"`          // "limit-orders"
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Difficulty    string   `json:"difficulty"`     // easy | medium | hard
	RequiredLevel *string  `json:"requiredLevel"`  // junior | middle | senior | null
	TimeLimit     int      `json:"timeLimit"`      // seconds
	Skills        []string `json:"skills"`
	DomainID      string   `json:"domainId"`
	ProjectID     string   `json:"projectId"`
}
