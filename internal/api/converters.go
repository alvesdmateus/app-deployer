package api

import (
	"github.com/mateus/app-deployer/internal/state"
)

// DeploymentToResponse converts a state.Deployment to DeploymentResponse
func DeploymentToResponse(d *state.Deployment) DeploymentResponse {
	return DeploymentResponse{
		ID:          d.ID,
		Name:        d.Name,
		AppName:     d.AppName,
		Version:     d.Version,
		Status:      d.Status,
		Cloud:       d.Cloud,
		Region:      d.Region,
		ExternalIP:  d.ExternalIP,
		ExternalURL: d.ExternalURL,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
		DeployedAt:  d.DeployedAt,
	}
}

// DeploymentsToResponse converts a slice of state.Deployment to DeploymentResponse
func DeploymentsToResponse(deployments []state.Deployment) []DeploymentResponse {
	responses := make([]DeploymentResponse, len(deployments))
	for i, d := range deployments {
		responses[i] = DeploymentToResponse(&d)
	}
	return responses
}

// InfrastructureToResponse converts state.Infrastructure to InfrastructureResponse
func InfrastructureToResponse(i *state.Infrastructure) InfrastructureResponse {
	return InfrastructureResponse{
		ID:           i.ID,
		DeploymentID: i.DeploymentID,
		ClusterName:  i.ClusterName,
		Namespace:    i.Namespace,
		ServiceName:  i.ServiceName,
		Status:       i.Status,
		Config:       i.Config,
		CreatedAt:    i.CreatedAt,
		UpdatedAt:    i.UpdatedAt,
	}
}

// BuildToResponse converts state.Build to BuildResponse
func BuildToResponse(b *state.Build) BuildResponse {
	return BuildResponse{
		ID:           b.ID,
		DeploymentID: b.DeploymentID,
		ImageTag:     b.ImageTag,
		Status:       b.Status,
		BuildLog:     b.BuildLog,
		StartedAt:    b.StartedAt,
		CompletedAt:  b.CompletedAt,
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
	}
}
