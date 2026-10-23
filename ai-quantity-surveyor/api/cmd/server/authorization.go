package main

import (
	"net/http"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/auth"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

const localOwnerID = "local"

func requestOwnerID(request *http.Request) string {
	if user, ok := auth.UserFromContext(request.Context()); ok {
		return user.ID
	}
	return localOwnerID
}

func projectForRequest(store project.Store, request *http.Request, projectID string) (project.Project, bool) {
	return store.GetProjectForOwner(projectID, requestOwnerID(request))
}

func projectOwnedByRequest(store project.Store, request *http.Request, projectID string) bool {
	_, found := projectForRequest(store, request, projectID)
	return found
}

func floorPlanForRequest(store project.Store, request *http.Request, floorPlanID string) (project.FloorPlan, bool) {
	floorPlan, found := store.GetFloorPlan(floorPlanID)
	if !found {
		return project.FloorPlan{}, false
	}
	if _, found = projectForRequest(store, request, floorPlan.ProjectID); !found {
		return project.FloorPlan{}, false
	}
	return floorPlan, true
}

func analysisProjectForRequest(store project.Store, analyses *analysis.Service, request *http.Request, analysisID string) (project.Project, bool) {
	candidate, found := analyses.Get(analysisID)
	if !found {
		return project.Project{}, false
	}
	floorPlan, found := floorPlanForRequest(store, request, candidate.FloorPlanID)
	if !found {
		return project.Project{}, false
	}
	return projectForRequest(store, request, floorPlan.ProjectID)
}
