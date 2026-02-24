package shared

import "errors"

// Tenant errors.
var (
	ErrTenantNotFound      = errors.New("tenant not found")
	ErrTenantCodeExists    = errors.New("tenant code already exists")
	ErrInvalidTenant       = errors.New("invalid tenant data")
	ErrMemberExists        = errors.New("user is already a member")
	ErrMemberNotFound      = errors.New("membership not found")
	ErrPlatformDomainOnly  = errors.New("operation requires platform domain")
	ErrParentTenantInvalid = errors.New("invalid parent tenant")
)
