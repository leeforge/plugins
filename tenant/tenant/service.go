package tenant

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/leeforge/framework/logging"
	"github.com/leeforge/framework/plugin"
	"go.uber.org/zap"

	"github.com/leeforge/core"
	coremod "github.com/leeforge/core/core"
	coreent "github.com/leeforge/core/server/ent"
	entTenant "github.com/leeforge/core/server/ent/tenant"
	"github.com/leeforge/core/server/ent/tenantuser"

	"github.com/leeforge/plugins/tenant/shared"
)

// Service handles tenant CRUD and membership operations.
type Service struct {
	client     *coreent.Client
	domainSvc  core.DomainWriter
	events     plugin.EventBus
	logger     logging.Logger
	roleSeeder shared.RoleSeeder
	userLookup shared.UserLookup
}

// NewService creates a new tenant service.
func NewService(
	client *coreent.Client,
	domainSvc core.DomainWriter,
	events plugin.EventBus,
	logger logging.Logger,
	roleSeeder shared.RoleSeeder,
	userLookup shared.UserLookup,
) *Service {
	return &Service{
		client:     client,
		domainSvc:  domainSvc,
		events:     events,
		logger:     logger,
		roleSeeder: roleSeeder,
		userLookup: userLookup,
	}
}

// Ping verifies database connectivity.
func (s *Service) Ping(ctx context.Context) error {
	if s.client == nil {
		return fmt.Errorf("database client not initialized")
	}
	_, err := s.client.Tenant.Query().Limit(1).All(ctx)
	return err
}

// CreateTenant creates a tenant, its domain, and owner membership.
func (s *Service) CreateTenant(ctx context.Context, req *CreateRequest) (*TenantDTO, error) {
	if err := requirePlatformDomain(ctx); err != nil {
		return nil, err
	}

	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" || name == "" {
		return nil, shared.ErrInvalidTenant
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("start transaction: %w", err)
	}

	builder := tx.Tenant.Create().
		SetCode(code).
		SetName(name)

	parentTenantID, hasParent, err := s.resolveParentTenantID(ctx, req.ParentTenantID, uuid.Nil)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if hasParent {
		builder.SetParentTenantID(parentTenantID)
	}

	if req.Description != "" {
		builder.SetDescription(req.Description)
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		builder.SetStatus(entTenant.Status(status))
	}

	var ownerID uuid.UUID
	var hasOwner bool
	if id, ok := core.GetUserID(ctx); ok {
		ownerID = id
		hasOwner = true
		builder.SetOwnerID(ownerID)
	}

	t, err := builder.Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		if coreent.IsConstraintError(err) {
			return nil, shared.ErrTenantCodeExists
		}
		return nil, fmt.Errorf("create tenant: %w", err)
	}

	// Create domain via DomainResolver (before seeding roles so we have the domainID).
	dom, err := s.domainSvc.EnsureDomain(ctx, "tenant", code, name)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("ensure domain: %w", err)
	}

	// Seed baseline roles for the tenant using domain ID.
	if err := s.roleSeeder.SeedBaselineRoles(ctx, dom.DomainID); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("seed baseline roles: %w", err)
	}

	// Bind owner membership.
	if hasOwner {
		if err := s.domainSvc.AddMembership(ctx, dom.DomainID, ownerID, "tenant_admin", true); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("add owner membership to domain: %w", err)
		}
		if err := s.ensureMembershipTx(ctx, tx, t.ID, ownerID, true, "tenant_admin"); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("create owner tenant-user record: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tenant creation: %w", err)
	}

	dto := s.toDTO(t, dom.DomainID)

	// Publish event.
	actorID := ownerID
	_ = s.events.Publish(ctx, plugin.Event{
		Name:   shared.EventTenantCreated,
		Source: "tenant",
		Data: shared.TenantEventData{
			TenantID:   t.ID,
			TenantCode: t.Code,
			DomainID:   dom.DomainID,
			ActorID:    actorID,
		},
	})

	return dto, nil
}

// ListTenants returns a paginated list of tenants.
func (s *Service) ListTenants(ctx context.Context, filters ListFilters) (*ListResult, error) {
	if err := requirePlatformDomain(ctx); err != nil {
		return nil, err
	}

	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PageSize < 1 {
		filters.PageSize = 20
	}
	if filters.PageSize > 100 {
		filters.PageSize = 100
	}

	query := s.client.Tenant.Query()
	if !filters.IncludeDeleted {
		query = query.Where(entTenant.DeletedAtIsNil())
	}
	if filters.Query != "" {
		search := strings.TrimSpace(filters.Query)
		query = query.Where(
			entTenant.Or(
				entTenant.CodeContainsFold(search),
				entTenant.NameContainsFold(search),
			),
		)
	}
	if filters.Status != "" {
		query = query.Where(entTenant.StatusEQ(entTenant.Status(filters.Status)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count tenants: %w", err)
	}

	offset := (filters.Page - 1) * filters.PageSize
	items, err := query.
		Offset(offset).
		Limit(filters.PageSize).
		Order(coreent.Desc(entTenant.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}

	dtos := make([]*TenantDTO, len(items))
	for i, item := range items {
		domainID := s.resolveDomainIDSafe(ctx, item.Code)
		dtos[i] = s.toDTO(item, domainID)
	}

	totalPages := (total + filters.PageSize - 1) / filters.PageSize
	return &ListResult{
		Tenants:    dtos,
		Total:      total,
		Page:       filters.Page,
		PageSize:   filters.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetTenant returns a single tenant by ID.
func (s *Service) GetTenant(ctx context.Context, id uuid.UUID) (*TenantDTO, error) {
	t, err := s.client.Tenant.Get(ctx, id)
	if err != nil {
		if coreent.IsNotFound(err) {
			return nil, shared.ErrTenantNotFound
		}
		return nil, fmt.Errorf("get tenant: %w", err)
	}
	domainID := s.resolveDomainIDSafe(ctx, t.Code)
	return s.toDTO(t, domainID), nil
}

// GetTenantByCode returns a single tenant by code.
func (s *Service) GetTenantByCode(ctx context.Context, code string) (*TenantDTO, error) {
	t, err := s.client.Tenant.Query().
		Where(entTenant.CodeEQ(code), entTenant.DeletedAtIsNil()).
		Only(ctx)
	if err != nil {
		if coreent.IsNotFound(err) {
			return nil, shared.ErrTenantNotFound
		}
		return nil, fmt.Errorf("get tenant by code: %w", err)
	}
	domainID := s.resolveDomainIDSafe(ctx, t.Code)
	return s.toDTO(t, domainID), nil
}

// UpdateTenant updates tenant fields.
func (s *Service) UpdateTenant(ctx context.Context, id uuid.UUID, req *UpdateRequest) (*TenantDTO, error) {
	if err := requirePlatformDomain(ctx); err != nil {
		return nil, err
	}

	t, err := s.client.Tenant.Get(ctx, id)
	if err != nil {
		if coreent.IsNotFound(err) {
			return nil, shared.ErrTenantNotFound
		}
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	updater := s.client.Tenant.UpdateOne(t)
	parentTenantID, hasParent, err := s.resolveParentTenantID(ctx, req.ParentTenantID, id)
	if err != nil {
		return nil, err
	}
	if hasParent {
		updater.SetParentTenantID(parentTenantID)
	}
	if req.Name != "" {
		updater.SetName(strings.TrimSpace(req.Name))
	}
	if req.Description != "" {
		updater.SetDescription(req.Description)
	}
	if req.Status != "" {
		updater.SetStatus(entTenant.Status(req.Status))
	}

	t, err = updater.Save(ctx)
	if err != nil {
		if coreent.IsNotFound(err) {
			return nil, shared.ErrTenantNotFound
		}
		return nil, fmt.Errorf("update tenant: %w", err)
	}

	domainID := s.resolveDomainIDSafe(ctx, t.Code)
	dto := s.toDTO(t, domainID)

	actorID, _ := core.GetUserID(ctx)
	_ = s.events.Publish(ctx, plugin.Event{
		Name:   shared.EventTenantUpdated,
		Source: "tenant",
		Data: shared.TenantEventData{
			TenantID:   t.ID,
			TenantCode: t.Code,
			DomainID:   domainID,
			ActorID:    actorID,
		},
	})

	return dto, nil
}

// DeleteTenant soft-deletes a tenant.
func (s *Service) DeleteTenant(ctx context.Context, id uuid.UUID) error {
	if err := requirePlatformDomain(ctx); err != nil {
		return err
	}

	t, err := s.client.Tenant.Get(ctx, id)
	if err != nil {
		if coreent.IsNotFound(err) {
			return shared.ErrTenantNotFound
		}
		return fmt.Errorf("get tenant: %w", err)
	}

	now := time.Now()
	if _, err := s.client.Tenant.UpdateOneID(id).SetDeletedAt(now).Save(ctx); err != nil {
		if coreent.IsNotFound(err) {
			return shared.ErrTenantNotFound
		}
		return fmt.Errorf("soft delete tenant: %w", err)
	}

	domainID := s.resolveDomainIDSafe(ctx, t.Code)
	actorID, _ := core.GetUserID(ctx)
	_ = s.events.Publish(ctx, plugin.Event{
		Name:   shared.EventTenantDeleted,
		Source: "tenant",
		Data: shared.TenantEventData{
			TenantID:   t.ID,
			TenantCode: t.Code,
			DomainID:   domainID,
			ActorID:    actorID,
		},
	})

	return nil
}

// AddMember adds a user to a tenant.
func (s *Service) AddMember(ctx context.Context, tenantID, userID uuid.UUID, role string) error {
	if err := requirePlatformDomain(ctx); err != nil {
		return err
	}

	t, err := s.client.Tenant.Get(ctx, tenantID)
	if err != nil {
		if coreent.IsNotFound(err) {
			return shared.ErrTenantNotFound
		}
		return fmt.Errorf("get tenant: %w", err)
	}

	// Check user exists via userLookup.
	u, err := s.userLookup.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	// Check username/email conflict within the tenant.
	existingMembers, err := s.client.TenantUser.Query().
		Where(
			tenantuser.TenantIDEQ(t.ID),
			tenantuser.DeletedAtIsNil(),
			tenantuser.StatusEQ(tenantuser.StatusActive),
		).
		WithUser().
		All(ctx)
	if err != nil {
		return fmt.Errorf("check member conflict: %w", err)
	}
	for _, m := range existingMembers {
		if m.Edges.User == nil || m.UserID == userID {
			continue
		}
		if m.Edges.User.Username == u.Username || m.Edges.User.Email == u.Email {
			return shared.ErrMemberExists
		}
	}

	if role == "" {
		role = "member"
	}

	// Add domain membership.
	domainID := s.resolveDomainIDSafe(ctx, t.Code)
	if domainID != uuid.Nil {
		if err := s.domainSvc.AddMembership(ctx, domainID, userID, role, false); err != nil {
			return fmt.Errorf("add domain membership: %w", err)
		}
	}

	// Create TenantUser record.
	if err := s.ensureMembership(ctx, t.ID, userID, false, role); err != nil {
		return fmt.Errorf("ensure membership: %w", err)
	}

	actorID, _ := core.GetUserID(ctx)
	_ = s.events.Publish(ctx, plugin.Event{
		Name:   shared.EventTenantMemberAdded,
		Source: "tenant",
		Data: shared.MemberEventData{
			TenantID: tenantID,
			UserID:   userID,
			Role:     role,
			ActorID:  actorID,
		},
	})

	return nil
}

// RemoveMember removes a user from a tenant.
func (s *Service) RemoveMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	if err := requirePlatformDomain(ctx); err != nil {
		return err
	}

	t, err := s.client.Tenant.Get(ctx, tenantID)
	if err != nil {
		if coreent.IsNotFound(err) {
			return shared.ErrTenantNotFound
		}
		return fmt.Errorf("get tenant: %w", err)
	}

	membership, err := s.client.TenantUser.Query().
		Where(
			tenantuser.TenantIDEQ(t.ID),
			tenantuser.UserID(userID),
			tenantuser.DeletedAtIsNil(),
		).
		First(ctx)
	if err != nil {
		if coreent.IsNotFound(err) {
			return shared.ErrMemberNotFound
		}
		return fmt.Errorf("get membership: %w", err)
	}

	now := time.Now()
	if _, err := s.client.TenantUser.Update().Where(tenantuser.ID(membership.ID)).SetDeletedAt(now).Save(ctx); err != nil {
		return fmt.Errorf("remove membership: %w", err)
	}

	// Remove domain membership.
	domainID := s.resolveDomainIDSafe(ctx, t.Code)
	if domainID != uuid.Nil {
		_ = s.domainSvc.RemoveMembership(ctx, domainID, userID)
	}

	// Reassign default if needed.
	if membership.IsDefault {
		alt, err := s.client.TenantUser.Query().
			Where(
				tenantuser.UserID(userID),
				tenantuser.DeletedAtIsNil(),
				tenantuser.StatusEQ(tenantuser.StatusActive),
			).
			Order(coreent.Asc(tenantuser.FieldCreatedAt)).
			First(ctx)
		if err == nil {
			_, _ = s.client.TenantUser.UpdateOneID(alt.ID).SetIsDefault(true).Save(ctx)
		}
	}

	actorID, _ := core.GetUserID(ctx)
	_ = s.events.Publish(ctx, plugin.Event{
		Name:   shared.EventTenantMemberRemoved,
		Source: "tenant",
		Data: shared.MemberEventData{
			TenantID: tenantID,
			UserID:   userID,
			ActorID:  actorID,
		},
	})

	return nil
}

// ListMembers returns a paginated list of tenant members.
func (s *Service) ListMembers(ctx context.Context, tenantID uuid.UUID, page, pageSize int) (*MemberListResult, error) {
	if err := requirePlatformDomain(ctx); err != nil {
		return nil, err
	}

	t, err := s.client.Tenant.Get(ctx, tenantID)
	if err != nil {
		if coreent.IsNotFound(err) {
			return nil, shared.ErrTenantNotFound
		}
		return nil, fmt.Errorf("get tenant: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := s.client.TenantUser.Query().
		Where(
			tenantuser.TenantIDEQ(t.ID),
			tenantuser.DeletedAtIsNil(),
			tenantuser.StatusEQ(tenantuser.StatusActive),
		).
		WithUser()

	total, err := query.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count members: %w", err)
	}

	offset := (page - 1) * pageSize
	items, err := query.
		Offset(offset).
		Limit(pageSize).
		Order(coreent.Desc(tenantuser.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	dtos := make([]*MemberDTO, 0, len(items))
	for _, item := range items {
		u := item.Edges.User
		if u == nil {
			continue
		}
		dtos = append(dtos, &MemberDTO{
			ID:        u.ID,
			Username:  u.Username,
			Email:     u.Email,
			Nickname:  u.Nickname,
			Status:    string(u.Status),
			Role:      item.Role,
			IsDefault: item.IsDefault,
		})
	}

	totalPages := (total + pageSize - 1) / pageSize
	return &MemberListResult{
		Members:    dtos,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// ListMyTenants returns the tenants the given user belongs to.
func (s *Service) ListMyTenants(ctx context.Context, userID uuid.UUID) (*MyTenantListResult, error) {
	ctxNoTenant := coremod.WithoutTenant(ctx)

	memberships, err := s.client.TenantUser.Query().
		Where(
			tenantuser.UserID(userID),
			tenantuser.DeletedAtIsNil(),
			tenantuser.StatusEQ(tenantuser.StatusActive),
		).
		Order(coreent.Desc(tenantuser.FieldIsDefault), coreent.Desc(tenantuser.FieldCreatedAt)).
		All(ctxNoTenant)
	if err != nil {
		return nil, fmt.Errorf("list memberships: %w", err)
	}

	if len(memberships) == 0 {
		return &MyTenantListResult{Tenants: []*MyTenantDTO{}}, nil
	}

	tenantIDs := make([]uuid.UUID, 0, len(memberships))
	idSet := make(map[uuid.UUID]struct{}, len(memberships))
	for _, m := range memberships {
		if m.TenantID == uuid.Nil {
			continue
		}
		if _, ok := idSet[m.TenantID]; !ok {
			idSet[m.TenantID] = struct{}{}
			tenantIDs = append(tenantIDs, m.TenantID)
		}
	}

	tenants, err := s.client.Tenant.Query().
		Where(entTenant.IDIn(tenantIDs...)).
		All(ctxNoTenant)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}

	tenantMap := make(map[uuid.UUID]*coreent.Tenant, len(tenants))
	for _, t := range tenants {
		tenantMap[t.ID] = t
	}

	results := make([]*MyTenantDTO, 0, len(memberships))
	for _, m := range memberships {
		t, ok := tenantMap[m.TenantID]
		if !ok {
			continue
		}
		results = append(results, &MyTenantDTO{
			ID:        t.ID,
			Code:      t.Code,
			Name:      t.Name,
			Status:    string(t.Status),
			Role:      m.Role,
			IsDefault: m.IsDefault,
		})
	}

	return &MyTenantListResult{Tenants: results}, nil
}

// IsMember reports whether a user is a member of the given tenant.
func (s *Service) IsMember(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	t, err := s.client.Tenant.Get(ctx, tenantID)
	if err != nil {
		if coreent.IsNotFound(err) {
			return false, shared.ErrTenantNotFound
		}
		return false, fmt.Errorf("get tenant: %w", err)
	}

	domainID := s.resolveDomainIDSafe(ctx, t.Code)
	if domainID != uuid.Nil {
		return s.domainSvc.CheckMembership(ctx, domainID, userID)
	}

	// Fallback: check TenantUser table directly.
	return s.client.TenantUser.Query().
		Where(
			tenantuser.TenantIDEQ(t.ID),
			tenantuser.UserID(userID),
			tenantuser.DeletedAtIsNil(),
			tenantuser.StatusEQ(tenantuser.StatusActive),
		).
		Exist(ctx)
}

// GetDomainID returns the domain ID for the given tenant code.
func (s *Service) GetDomainID(ctx context.Context, tenantCode string) (uuid.UUID, error) {
	dom, err := s.domainSvc.ResolveDomain(ctx, "tenant", tenantCode)
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve domain: %w", err)
	}
	return dom.DomainID, nil
}

// OnUserDeleted cleans up memberships when a user is deleted.
func (s *Service) OnUserDeleted(ctx context.Context, data any) error {
	type userDeletedPayload struct {
		UserID uuid.UUID `json:"userId"`
	}

	payload, ok := data.(*userDeletedPayload)
	if !ok {
		s.logger.Warn("tenant: ignoring unrecognized user.deleted payload")
		return nil
	}

	memberships, err := s.client.TenantUser.Query().
		Where(
			tenantuser.UserID(payload.UserID),
			tenantuser.DeletedAtIsNil(),
		).
		All(ctx)
	if err != nil {
		return fmt.Errorf("list user memberships: %w", err)
	}

	now := time.Now()
	for _, m := range memberships {
		if _, err := s.client.TenantUser.Update().Where(tenantuser.ID(m.ID)).SetDeletedAt(now).Save(ctx); err != nil {
			s.logger.Error("tenant: failed to remove membership on user delete",
				zap.Stringer("tenantID", m.TenantID),
				zap.Stringer("userID", payload.UserID),
				zap.Error(err),
			)
		}
	}

	return nil
}

// --- private helpers ---

func requirePlatformDomain(ctx context.Context) error {
	ac := coremod.GetActingContext(ctx)
	if ac == nil || !ac.IsPlatformDomain() {
		return shared.ErrPlatformDomainOnly
	}
	return nil
}

func (s *Service) toDTO(t *coreent.Tenant, domainID uuid.UUID) *TenantDTO {
	dto := &TenantDTO{
		ID:          t.ID,
		Code:        t.Code,
		Name:        t.Name,
		Description: t.Description,
		Status:      string(t.Status),
		DomainID:    domainID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
	if t.OwnerID != uuid.Nil {
		ownerID := t.OwnerID
		dto.OwnerID = &ownerID
	}
	if t.ParentTenantID != nil && *t.ParentTenantID != uuid.Nil {
		parentTenantID := *t.ParentTenantID
		dto.ParentTenantID = &parentTenantID
	}
	return dto
}

func (s *Service) resolveDomainIDSafe(ctx context.Context, tenantCode string) uuid.UUID {
	dom, err := s.domainSvc.ResolveDomain(ctx, "tenant", tenantCode)
	if err != nil {
		return uuid.Nil
	}
	return dom.DomainID
}

func (s *Service) resolveParentTenantID(ctx context.Context, parentRef string, selfID uuid.UUID) (uuid.UUID, bool, error) {
	parentRef = strings.TrimSpace(parentRef)
	if parentRef == "" {
		return uuid.Nil, false, nil
	}

	var parentEntity *coreent.Tenant
	var err error
	if parentID, parseErr := uuid.Parse(parentRef); parseErr == nil {
		parentEntity, err = s.client.Tenant.Query().
			Where(entTenant.ID(parentID), entTenant.DeletedAtIsNil()).
			Only(ctx)
	} else {
		parentEntity, err = s.client.Tenant.Query().
			Where(entTenant.CodeEQ(parentRef), entTenant.DeletedAtIsNil()).
			Only(ctx)
	}
	if err != nil {
		if coreent.IsNotFound(err) {
			return uuid.Nil, false, shared.ErrParentTenantInvalid
		}
		return uuid.Nil, false, fmt.Errorf("resolve parent tenant: %w", err)
	}

	if selfID != uuid.Nil && parentEntity.ID == selfID {
		return uuid.Nil, false, shared.ErrParentTenantInvalid
	}

	return parentEntity.ID, true, nil
}

func (s *Service) ensureMembership(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, forceDefault bool, roleName string) error {
	existing, err := s.client.TenantUser.Query().
		Where(
			tenantuser.TenantIDEQ(tenantID),
			tenantuser.UserID(userID),
		).
		First(ctx)
	if err == nil {
		if existing.DeletedAt.IsZero() && existing.Status == tenantuser.StatusActive {
			return nil
		}
		_, err = s.client.TenantUser.UpdateOneID(existing.ID).
			ClearDeletedAt().
			SetStatus(tenantuser.StatusActive).
			Save(ctx)
		return err
	}
	if !coreent.IsNotFound(err) {
		return fmt.Errorf("query membership: %w", err)
	}

	isDefault := forceDefault
	if !isDefault {
		hasDefault, err := s.client.TenantUser.Query().
			Where(
				tenantuser.UserID(userID),
				tenantuser.IsDefault(true),
				tenantuser.DeletedAtIsNil(),
			).
			Exist(ctx)
		if err != nil {
			return fmt.Errorf("check default tenant: %w", err)
		}
		isDefault = !hasDefault
	}

	builder := s.client.TenantUser.Create().
		SetTenantID(tenantID).
		SetUserID(userID).
		SetStatus(tenantuser.StatusActive).
		SetIsDefault(isDefault)
	if roleName != "" {
		builder.SetRole(roleName)
	}
	_, err = builder.Save(ctx)
	if err != nil {
		return fmt.Errorf("create membership: %w", err)
	}
	return nil
}

func (s *Service) ensureMembershipTx(ctx context.Context, tx *coreent.Tx, tenantID uuid.UUID, userID uuid.UUID, forceDefault bool, roleName string) error {
	existing, err := tx.TenantUser.Query().
		Where(
			tenantuser.TenantIDEQ(tenantID),
			tenantuser.UserID(userID),
		).
		First(ctx)
	if err == nil {
		updater := tx.TenantUser.UpdateOneID(existing.ID).
			ClearDeletedAt().
			SetStatus(tenantuser.StatusActive)
		if forceDefault {
			updater.SetIsDefault(true)
		}
		if roleName != "" {
			updater.SetRole(roleName)
		}
		_, err = updater.Save(ctx)
		return err
	}
	if !coreent.IsNotFound(err) {
		return fmt.Errorf("query membership: %w", err)
	}

	isDefault := forceDefault
	if !isDefault {
		hasDefault, err := tx.TenantUser.Query().
			Where(
				tenantuser.UserID(userID),
				tenantuser.IsDefault(true),
				tenantuser.DeletedAtIsNil(),
			).
			Exist(ctx)
		if err != nil {
			return fmt.Errorf("check default tenant: %w", err)
		}
		isDefault = !hasDefault
	}

	builder := tx.TenantUser.Create().
		SetTenantID(tenantID).
		SetUserID(userID).
		SetStatus(tenantuser.StatusActive).
		SetIsDefault(isDefault)
	if roleName != "" {
		builder.SetRole(roleName)
	}
	_, err = builder.Save(ctx)
	if err != nil {
		return fmt.Errorf("create membership: %w", err)
	}
	return nil
}
