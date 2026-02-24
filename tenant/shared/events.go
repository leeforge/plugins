package shared

import "github.com/google/uuid"

// Event topic constants.
const (
	EventTenantCreated       = "tenant.created"
	EventTenantUpdated       = "tenant.updated"
	EventTenantDeleted       = "tenant.deleted"
	EventTenantMemberAdded   = "tenant.member.added"
	EventTenantMemberRemoved = "tenant.member.removed"
)

// TenantEventData is the payload for tenant lifecycle events.
type TenantEventData struct {
	TenantID   uuid.UUID `json:"tenantId"`
	TenantCode string    `json:"tenantCode"`
	DomainID   uuid.UUID `json:"domainId"`
	ActorID    uuid.UUID `json:"actorId"`
}

// MemberEventData is the payload for membership events.
type MemberEventData struct {
	TenantID uuid.UUID `json:"tenantId"`
	UserID   uuid.UUID `json:"userId"`
	Role     string    `json:"role"`
	ActorID  uuid.UUID `json:"actorId"`
}
