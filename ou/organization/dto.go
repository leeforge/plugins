package organization

import "github.com/google/uuid"

type CreateOrganizationRequest struct {
	Code     string     `json:"code" binding:"required"`
	Name     string     `json:"name" binding:"required"`
	ParentID *uuid.UUID `json:"parentId,omitempty"`
}

type AddOrganizationMemberRequest struct {
	UserID    uuid.UUID `json:"userId" binding:"required"`
	IsPrimary bool      `json:"isPrimary"`
}

type OrganizationResponse struct {
	ID       uuid.UUID  `json:"id"`
	DomainID uuid.UUID  `json:"domainId"`
	ParentID *uuid.UUID `json:"parentId,omitempty"`
	Code     string     `json:"code"`
	Name     string     `json:"name"`
	Path     string     `json:"path"`
}

type OrganizationTreeNode struct {
	ID       uuid.UUID               `json:"id"`
	DomainID uuid.UUID               `json:"domainId"`
	ParentID *uuid.UUID              `json:"parentId,omitempty"`
	Code     string                  `json:"code"`
	Name     string                  `json:"name"`
	Path     string                  `json:"path"`
	Children []*OrganizationTreeNode `json:"children,omitempty"`
}

type OrganizationMemberResponse struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`
	UserID         uuid.UUID `json:"userId"`
	IsPrimary      bool      `json:"isPrimary"`
}
