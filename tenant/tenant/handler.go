package tenant

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"

	"github.com/leeforge/core"
	"github.com/leeforge/core/server/httplog"

	"github.com/leeforge/plugins/tenant/shared"
)

// Handler handles tenant HTTP requests.
type Handler struct {
	service *Service
	logger  logging.Logger
}

// NewHandler creates a new tenant handler.
func NewHandler(service *Service, logger logging.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// CreateTenant handles POST /tenants
//
// @Summary Create tenant
// @Tags TenantPlugin-Tenants
// @Accept json
// @Produce json
// @Param body body CreateRequest true "Tenant payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants [post]
func (h *Handler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	result, err := h.service.CreateTenant(r.Context(), &req)
	if err != nil {
		h.mapTenantError(w, r, "Failed to create tenant", err)
		return
	}

	responder.OK(w, r, result)
}

// ListTenants handles GET /tenants
//
// @Summary List tenants
// @Tags TenantPlugin-Tenants
// @Produce json
// @Param page query int false "Page number"
// @Param pageSize query int false "Page size"
// @Param query query string false "Search query"
// @Param status query string false "Tenant status"
// @Param includeDeleted query bool false "Include deleted"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants [get]
func (h *Handler) ListTenants(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	filters := ListFilters{
		Page:           page,
		PageSize:       pageSize,
		Query:          r.URL.Query().Get("query"),
		Status:         r.URL.Query().Get("status"),
		IncludeDeleted: r.URL.Query().Get("includeDeleted") == "true",
	}

	result, err := h.service.ListTenants(r.Context(), filters)
	if err != nil {
		if errors.Is(err, shared.ErrPlatformDomainOnly) {
			responder.Forbidden(w, r, "Platform domain required")
			return
		}
		httplog.Error(h.logger, r, "Failed to list tenants", err)
		responder.DatabaseError(w, r, "Failed to list tenants")
		return
	}

	responder.OK(w, r, result)
}

// ListMyTenants handles GET /tenants/me
//
// @Summary List my tenants
// @Tags TenantPlugin-Tenants
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/me [get]
func (h *Handler) ListMyTenants(w http.ResponseWriter, r *http.Request) {
	userID, ok := core.GetUserID(r.Context())
	if !ok {
		responder.Unauthorized(w, r, "Missing user context")
		return
	}

	result, err := h.service.ListMyTenants(r.Context(), userID)
	if err != nil {
		httplog.Error(h.logger, r, "Failed to list my tenants", err)
		responder.DatabaseError(w, r, "Failed to list my tenants")
		return
	}

	responder.OK(w, r, result)
}

// GetTenant handles GET /tenants/{id}
//
// @Summary Get tenant
// @Tags TenantPlugin-Tenants
// @Produce json
// @Param id path string true "Tenant ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/{id} [get]
func (h *Handler) GetTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid tenant ID")
		return
	}

	result, err := h.service.GetTenant(r.Context(), tenantID)
	if err != nil {
		h.mapTenantError(w, r, "Failed to get tenant", err)
		return
	}

	responder.OK(w, r, result)
}

// UpdateTenant handles PUT /tenants/{id}
//
// @Summary Update tenant
// @Tags TenantPlugin-Tenants
// @Accept json
// @Produce json
// @Param id path string true "Tenant ID"
// @Param body body UpdateRequest true "Tenant update payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/{id} [put]
func (h *Handler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid tenant ID")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	result, err := h.service.UpdateTenant(r.Context(), tenantID, &req)
	if err != nil {
		h.mapTenantError(w, r, "Failed to update tenant", err)
		return
	}

	responder.OK(w, r, result)
}

// DeleteTenant handles DELETE /tenants/{id}
//
// @Summary Delete tenant
// @Tags TenantPlugin-Tenants
// @Param id path string true "Tenant ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/{id} [delete]
func (h *Handler) DeleteTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid tenant ID")
		return
	}

	if err := h.service.DeleteTenant(r.Context(), tenantID); err != nil {
		h.mapTenantError(w, r, "Failed to delete tenant", err)
		return
	}

	responder.OK(w, r, map[string]string{"message": "Tenant deleted successfully"})
}

// AddMember handles POST /tenants/{id}/members
//
// @Summary Add tenant member
// @Tags TenantPlugin-Tenants
// @Accept json
// @Produce json
// @Param id path string true "Tenant ID"
// @Param body body AddMemberRequest true "Member payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/{id}/members [post]
func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid tenant ID")
		return
	}

	var req AddMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		responder.BadRequest(w, r, "Invalid user ID")
		return
	}

	if err := h.service.AddMember(r.Context(), tenantID, userID, req.Role); err != nil {
		switch {
		case errors.Is(err, shared.ErrPlatformDomainOnly):
			responder.Forbidden(w, r, "Platform domain required")
		case errors.Is(err, shared.ErrTenantNotFound):
			responder.NotFound(w, r, "Tenant not found")
		case errors.Is(err, shared.ErrMemberExists):
			responder.Conflict(w, r, "User is already a member")
		default:
			httplog.Error(h.logger, r, "Failed to add member", err)
			responder.DatabaseError(w, r, "Failed to add member")
		}
		return
	}

	responder.OK(w, r, map[string]string{"message": "Member added successfully"})
}

// ListMembers handles GET /tenants/{id}/members
//
// @Summary List tenant members
// @Tags TenantPlugin-Tenants
// @Produce json
// @Param id path string true "Tenant ID"
// @Param page query int false "Page number"
// @Param pageSize query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/{id}/members [get]
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid tenant ID")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	result, err := h.service.ListMembers(r.Context(), tenantID, page, pageSize)
	if err != nil {
		if errors.Is(err, shared.ErrPlatformDomainOnly) {
			responder.Forbidden(w, r, "Platform domain required")
			return
		}
		if errors.Is(err, shared.ErrTenantNotFound) {
			responder.NotFound(w, r, "Tenant not found")
			return
		}
		httplog.Error(h.logger, r, "Failed to list members", err)
		responder.DatabaseError(w, r, "Failed to list members")
		return
	}

	responder.OK(w, r, result)
}

// RemoveMember handles DELETE /tenants/{id}/members/{userId}
//
// @Summary Remove tenant member
// @Tags TenantPlugin-Tenants
// @Param id path string true "Tenant ID"
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/tenants/{id}/members/{userId} [delete]
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid tenant ID")
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid user ID")
		return
	}

	if err := h.service.RemoveMember(r.Context(), tenantID, userID); err != nil {
		switch {
		case errors.Is(err, shared.ErrPlatformDomainOnly):
			responder.Forbidden(w, r, "Platform domain required")
		case errors.Is(err, shared.ErrTenantNotFound):
			responder.NotFound(w, r, "Tenant not found")
		case errors.Is(err, shared.ErrMemberNotFound):
			responder.NotFound(w, r, "Membership not found")
		default:
			httplog.Error(h.logger, r, "Failed to remove member", err)
			responder.DatabaseError(w, r, "Failed to remove member")
		}
		return
	}

	responder.OK(w, r, map[string]string{"message": "Member removed successfully"})
}

// mapTenantError maps common tenant service errors to HTTP responses.
func (h *Handler) mapTenantError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	switch {
	case errors.Is(err, shared.ErrTenantNotFound):
		responder.NotFound(w, r, "Tenant not found")
	case errors.Is(err, shared.ErrTenantCodeExists):
		responder.Conflict(w, r, "Tenant code already exists")
	case errors.Is(err, shared.ErrInvalidTenant), errors.Is(err, shared.ErrParentTenantInvalid):
		responder.BadRequest(w, r, "Invalid tenant data")
	case errors.Is(err, shared.ErrPlatformDomainOnly):
		responder.Forbidden(w, r, "Platform domain required")
	default:
		httplog.Error(h.logger, r, msg, err)
		responder.DatabaseError(w, r, msg)
	}
}
