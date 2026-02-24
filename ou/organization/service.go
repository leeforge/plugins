package organization

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/leeforge/core/server/ent"
	organizationEnt "github.com/leeforge/core/server/ent/organization"
	organizationMemberEnt "github.com/leeforge/core/server/ent/organizationmember"

	"github.com/leeforge/core/core"
)

var (
	ErrDomainContextMissing = errors.New("ou organization: missing domain context")
	ErrInvalidDomainID      = errors.New("ou organization: invalid domain id")
	ErrOrganizationNotFound = errors.New("ou organization: organization not found")
	ErrMemberAlreadyExists  = errors.New("ou organization: member already exists")
)

type Service struct {
	client *ent.Client
}

func NewService(client *ent.Client) *Service {
	return &Service{client: client}
}

func (s *Service) CreateOrganization(ctx context.Context, req *CreateOrganizationRequest) (*OrganizationResponse, error) {
	if req == nil {
		return nil, errors.New("ou organization: request is nil")
	}
	domainID, err := domainIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" || name == "" {
		return nil, errors.New("ou organization: code and name are required")
	}

	create := s.client.Organization.Create().
		SetDomainID(domainID).
		SetCode(code).
		SetName(name)

	pathPrefix := ""
	if req.ParentID != nil {
		parent, err := s.client.Organization.Query().
			Where(
				organizationEnt.IDEQ(*req.ParentID),
				organizationEnt.DomainIDEQ(domainID),
			).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return nil, ErrOrganizationNotFound
			}
			return nil, err
		}
		create.SetParentID(parent.ID)
		pathPrefix = parent.Path
	}

	path := code
	if pathPrefix != "" {
		path = pathPrefix + "/" + code
	}

	item, err := create.SetPath(path).Save(ctx)
	if err != nil {
		return nil, err
	}
	return toOrganizationResponse(item), nil
}

func (s *Service) GetOrganizationTree(ctx context.Context) ([]*OrganizationTreeNode, error) {
	domainID, err := domainIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	orgs, err := s.client.Organization.Query().
		Where(organizationEnt.DomainIDEQ(domainID)).
		Order(ent.Asc(organizationEnt.FieldPath)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	nodes := make(map[uuid.UUID]*OrganizationTreeNode, len(orgs))
	roots := make([]*OrganizationTreeNode, 0, len(orgs))

	for _, item := range orgs {
		nodes[item.ID] = &OrganizationTreeNode{
			ID:       item.ID,
			DomainID: item.DomainID,
			ParentID: item.ParentID,
			Code:     item.Code,
			Name:     item.Name,
			Path:     item.Path,
		}
	}

	for _, item := range orgs {
		node := nodes[item.ID]
		if item.ParentID == nil {
			roots = append(roots, node)
			continue
		}
		parent, ok := nodes[*item.ParentID]
		if !ok {
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}

	return roots, nil
}

func (s *Service) AddOrganizationMember(
	ctx context.Context,
	organizationID uuid.UUID,
	req *AddOrganizationMemberRequest,
) (*OrganizationMemberResponse, error) {
	if req == nil {
		return nil, errors.New("ou organization: request is nil")
	}
	domainID, err := domainIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.UserID == uuid.Nil {
		return nil, errors.New("ou organization: user id is required")
	}

	_, err = s.client.Organization.Query().
		Where(
			organizationEnt.IDEQ(organizationID),
			organizationEnt.DomainIDEQ(domainID),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}

	if req.IsPrimary {
		if err := s.client.OrganizationMember.Update().
			Where(
				organizationMemberEnt.DomainIDEQ(domainID),
				organizationMemberEnt.UserIDEQ(req.UserID),
				organizationMemberEnt.IsPrimaryEQ(true),
			).
			SetIsPrimary(false).
			Exec(ctx); err != nil {
			return nil, err
		}
	}

	item, err := s.client.OrganizationMember.Create().
		SetDomainID(domainID).
		SetOrganizationID(organizationID).
		SetUserID(req.UserID).
		SetIsPrimary(req.IsPrimary).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, ErrMemberAlreadyExists
		}
		return nil, err
	}
	return &OrganizationMemberResponse{
		ID:             item.ID,
		OrganizationID: item.OrganizationID,
		UserID:         item.UserID,
		IsPrimary:      item.IsPrimary,
	}, nil
}

func (s *Service) GetPrimaryOrganizationID(ctx context.Context, domainID, userID uuid.UUID) (uuid.UUID, error) {
	primary, err := s.client.OrganizationMember.Query().
		Where(
			organizationMemberEnt.DomainIDEQ(domainID),
			organizationMemberEnt.UserIDEQ(userID),
			organizationMemberEnt.IsPrimaryEQ(true),
		).
		Only(ctx)
	if err == nil {
		return primary.OrganizationID, nil
	}
	if !ent.IsNotFound(err) {
		return uuid.Nil, err
	}

	anyMember, err := s.client.OrganizationMember.Query().
		Where(
			organizationMemberEnt.DomainIDEQ(domainID),
			organizationMemberEnt.UserIDEQ(userID),
		).
		First(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return anyMember.OrganizationID, nil
}

func (s *Service) ListOrganizationUserIDs(ctx context.Context, domainID, orgID uuid.UUID) ([]uuid.UUID, error) {
	members, err := s.client.OrganizationMember.Query().
		Where(
			organizationMemberEnt.DomainIDEQ(domainID),
			organizationMemberEnt.OrganizationIDEQ(orgID),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return uniqueUserIDs(members), nil
}

func (s *Service) ListSubtreeUserIDs(ctx context.Context, domainID, orgID uuid.UUID) ([]uuid.UUID, error) {
	org, err := s.client.Organization.Query().
		Where(
			organizationEnt.IDEQ(orgID),
			organizationEnt.DomainIDEQ(domainID),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}

	nodes, err := s.client.Organization.Query().
		Where(
			organizationEnt.DomainIDEQ(domainID),
			organizationEnt.PathHasPrefix(org.Path),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return []uuid.UUID{}, nil
	}

	orgIDs := make([]uuid.UUID, 0, len(nodes))
	for _, node := range nodes {
		orgIDs = append(orgIDs, node.ID)
	}

	members, err := s.client.OrganizationMember.Query().
		Where(
			organizationMemberEnt.DomainIDEQ(domainID),
			organizationMemberEnt.OrganizationIDIn(orgIDs...),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return uniqueUserIDs(members), nil
}

func domainIDFromContext(ctx context.Context) (uuid.UUID, error) {
	rawDomainID, ok := core.GetDomainID(ctx)
	if !ok {
		return uuid.Nil, ErrDomainContextMissing
	}
	domainID, err := uuid.Parse(rawDomainID)
	if err != nil {
		return uuid.Nil, ErrInvalidDomainID
	}
	return domainID, nil
}

func toOrganizationResponse(item *ent.Organization) *OrganizationResponse {
	if item == nil {
		return nil
	}
	return &OrganizationResponse{
		ID:       item.ID,
		DomainID: item.DomainID,
		ParentID: item.ParentID,
		Code:     item.Code,
		Name:     item.Name,
		Path:     item.Path,
	}
}

func uniqueUserIDs(members ent.OrganizationMembers) []uuid.UUID {
	if len(members) == 0 {
		return []uuid.UUID{}
	}
	seen := make(map[uuid.UUID]struct{}, len(members))
	userIDs := make([]uuid.UUID, 0, len(members))
	for _, item := range members {
		if item == nil {
			continue
		}
		if _, ok := seen[item.UserID]; ok {
			continue
		}
		seen[item.UserID] = struct{}{}
		userIDs = append(userIDs, item.UserID)
	}
	return userIDs
}
