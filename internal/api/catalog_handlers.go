package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Catalog handlers — hierarchical browsing of domains/projects/tasks

func (s *Server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	domains := s.templateLoader.ListDomains()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"domains": domains,
		"total":   len(domains),
	})
}

func (s *Server) handleGetDomain(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainId")
	domain := s.templateLoader.GetDomain(domainID)
	if domain == nil {
		respondError(w, http.StatusNotFound, "not_found", "domain not found")
		return
	}
	respondJSON(w, http.StatusOK, domain)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainId")
	if s.templateLoader.GetDomain(domainID) == nil {
		respondError(w, http.StatusNotFound, "not_found", "domain not found")
		return
	}
	projects := s.templateLoader.ListProjects(domainID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"projects": projects,
		"total":    len(projects),
	})
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainId")
	projectName := chi.URLParam(r, "projectName")
	projectID := domainID + "/" + projectName

	project := s.templateLoader.GetProject(projectID)
	if project == nil {
		respondError(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	respondJSON(w, http.StatusOK, project)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainId")
	projectName := chi.URLParam(r, "projectName")
	projectID := domainID + "/" + projectName

	if s.templateLoader.GetProject(projectID) == nil {
		respondError(w, http.StatusNotFound, "not_found", "project not found")
		return
	}

	tasks := s.templateLoader.ListTasks(projectID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	domainID := chi.URLParam(r, "domainId")
	projectName := chi.URLParam(r, "projectName")
	taskCode := chi.URLParam(r, "taskCode")
	taskID := domainID + "/" + projectName + "/" + taskCode

	task := s.templateLoader.GetTask(taskID)
	if task == nil {
		respondError(w, http.StatusNotFound, "not_found", "task not found")
		return
	}
	respondJSON(w, http.StatusOK, task)
}
