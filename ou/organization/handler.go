package organization

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/leeforge/core/server/httplog"
	"github.com/leeforge/framework/http/responder"
	"github.com/leeforge/framework/logging"
)

type Handler struct {
	service *Service
	logger  logging.Logger
}

func NewHandler(service *Service, logger logging.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// CreateOrganization handles POST /ou/organizations
//
// @Summary Create organization
// @Tags OUPlugin-Organizations
// @Accept json
// @Produce json
// @Param body body CreateOrganizationRequest true "Organization payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/ou/organizations [post]
func (h *Handler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	var req CreateOrganizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	result, err := h.service.CreateOrganization(r.Context(), &req)
	if err != nil {
		h.mapServiceError(w, r, err)
		return
	}
	responder.OK(w, r, result)
}

// GetOrganizationTree handles GET /ou/organizations/tree
//
// @Summary Get organization tree
// @Tags OUPlugin-Organizations
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/ou/organizations/tree [get]
func (h *Handler) GetOrganizationTree(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.GetOrganizationTree(r.Context())
	if err != nil {
		h.mapServiceError(w, r, err)
		return
	}
	responder.OK(w, r, result)
}

// AddOrganizationMember handles POST /ou/organizations/{id}/members
//
// @Summary Add organization member
// @Tags OUPlugin-Organizations
// @Accept json
// @Produce json
// @Param id path string true "Organization ID"
// @Param body body AddOrganizationMemberRequest true "Organization member payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/ou/organizations/{id}/members [post]
func (h *Handler) AddOrganizationMember(w http.ResponseWriter, r *http.Request) {
	organizationID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		responder.BadRequest(w, r, "Invalid organization ID")
		return
	}

	var req AddOrganizationMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responder.BindError(w, r, nil)
		return
	}

	result, err := h.service.AddOrganizationMember(r.Context(), organizationID, &req)
	if err != nil {
		h.mapServiceError(w, r, err)
		return
	}
	responder.OK(w, r, result)
}

func (h *Handler) mapServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrDomainContextMissing):
		responder.BadRequest(w, r, "Missing domain context")
	case errors.Is(err, ErrInvalidDomainID):
		responder.BadRequest(w, r, "Invalid domain context")
	case errors.Is(err, ErrOrganizationNotFound):
		responder.NotFound(w, r, "Organization not found")
	case errors.Is(err, ErrMemberAlreadyExists):
		responder.Conflict(w, r, "Organization member already exists")
	default:
		httplog.Error(h.logger, r, "OU organization operation failed", err)
		responder.DatabaseError(w, r, "OU organization operation failed")
	}
}
