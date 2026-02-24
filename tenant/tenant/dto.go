package tenant

import (
	"time"

	"github.com/google/uuid"
)

// --- Requests ---

// CreateRequest is the input for creating a new tenant.
type CreateRequest struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	Status         string `json:"status,omitempty"`
	ParentTenantID string `json:"parentTenantId,omitempty"`
}

// UpdateRequest is the input for updating a tenant.
type UpdateRequest struct {
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	Status         string `json:"status,omitempty"`
	ParentTenantID string `json:"parentTenantId,omitempty"`
}

// AddMemberRequest is the input for adding a member to a tenant.
type AddMemberRequest struct {
	UserID string `json:"userId"`
	Role   string `json:"role,omitempty"`
}

// ListFilters holds query parameters for listing tenants.
type ListFilters struct {
	Page           int    `json:"page,omitempty"`
	PageSize       int    `json:"pageSize,omitempty"`
	Query          string `json:"query,omitempty"`
	Status         string `json:"status,omitempty"`
	IncludeDeleted bool   `json:"includeDeleted,omitempty"`
}

// --- Responses ---

// TenantDTO is the tenant representation returned by the API.
type TenantDTO struct {
	ID             uuid.UUID  `json:"id"`
	Code           string     `json:"code"`
	Name           string     `json:"name"`
	Description    string     `json:"description,omitempty"`
	Status         string     `json:"status"`
	OwnerID        *uuid.UUID `json:"ownerId,omitempty"`
	ParentTenantID *uuid.UUID `json:"parentTenantId,omitempty"`
	DomainID       uuid.UUID  `json:"domainId"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// ListResult is the paginated tenant list response.
type ListResult struct {
	Tenants    []*TenantDTO `json:"tenants"`
	Total      int          `json:"total"`
	Page       int          `json:"page"`
	PageSize   int          `json:"pageSize"`
	TotalPages int          `json:"totalPages"`
}

// MemberDTO is the tenant member representation.
type MemberDTO struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Nickname  string    `json:"nickname"`
	Status    string    `json:"status"`
	Role      string    `json:"role,omitempty"`
	IsDefault bool      `json:"isDefault"`
}

// MemberListResult is the paginated member list response.
type MemberListResult struct {
	Members    []*MemberDTO `json:"members"`
	Total      int          `json:"total"`
	Page       int          `json:"page"`
	PageSize   int          `json:"pageSize"`
	TotalPages int          `json:"totalPages"`
}

// MyTenantDTO is a summary of a tenant the current user belongs to.
type MyTenantDTO struct {
	ID        uuid.UUID `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Role      string    `json:"role,omitempty"`
	IsDefault bool      `json:"isDefault"`
}

// MyTenantListResult is the list of tenants for the current user.
type MyTenantListResult struct {
	Tenants []*MyTenantDTO `json:"tenants"`
}
