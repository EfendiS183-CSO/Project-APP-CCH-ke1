package breadcrumbs

import (
	"context"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type Service struct {
	accessService *authorization.AccessService
	repository    Repository
}

func NewService(accessService *authorization.AccessService, repository Repository) *Service {
	return &Service{accessService: accessService, repository: repository}
}

func (service *Service) List(ctx context.Context, identity authentication.VerifiedIdentity, query ListQuery) (Page, error) {
	if service.accessService == nil || service.repository == nil {
		return Page{}, ErrServiceNotReady
	}
	query, err := query.Normalize()
	if err != nil {
		return Page{}, err
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return Page{}, err
	}
	return service.repository.List(ctx, query)
}

func (service *Service) Get(ctx context.Context, identity authentication.VerifiedIdentity, pageTitle string) (Breadcrumb, error) {
	if service.accessService == nil || service.repository == nil {
		return Breadcrumb{}, ErrServiceNotReady
	}
	title, err := ValidateTitle(pageTitle)
	if err != nil {
		return Breadcrumb{}, err
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionView); err != nil {
		return Breadcrumb{}, err
	}
	return service.repository.Get(ctx, title)
}

func (service *Service) Check(ctx context.Context, identity authentication.VerifiedIdentity, pageTitle string) ([]Ancestor, error) {
	if service.accessService == nil || service.repository == nil {
		return nil, ErrServiceNotReady
	}
	title, err := ValidateTitle(pageTitle)
	if err != nil {
		return nil, err
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionList); err != nil {
		return nil, err
	}
	return service.repository.Ancestors(ctx, title)
}

func (service *Service) Add(ctx context.Context, identity authentication.VerifiedIdentity, parentTitle, pageTitle, pageURL string) (Breadcrumb, error) {
	if service.accessService == nil || service.repository == nil {
		return Breadcrumb{}, ErrServiceNotReady
	}
	title, err := ValidateTitle(pageTitle)
	if err != nil {
		return Breadcrumb{}, err
	}
	url, err := ValidateURL(pageURL)
	if err != nil {
		return Breadcrumb{}, err
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionAdd); err != nil {
		return Breadcrumb{}, err
	}
	parent, err := ValidateTitle(parentTitle)
	if err != nil && parentTitle != "" {
		return Breadcrumb{}, err
	}
	var result Breadcrumb
	err = service.repository.Transaction(ctx, func(ctx context.Context, q Querier) error {
		if exists, err := q.Exists(ctx, title); err != nil {
			return err
		} else if exists {
			return ErrDuplicateTitle
		}
		total, err := q.Count(ctx)
		if err != nil {
			return err
		}
		left, right := 1, 2
		if total > 0 {
			if err := validateParent(ctx, q, parent); err != nil {
				return err
			}
			parentNode, err := q.Get(ctx, parent)
			if err != nil {
				return err
			}
			left, right = parentNode.Right, parentNode.Right+1
		} else if parent != "" {
			return ErrInvalidParent
		}
		if err := q.Add(ctx, title, url, parent, left, right); err != nil {
			return err
		}
		result = Breadcrumb{PageTitle: title, PageURL: url, Left: left, Right: right}
		return nil
	})
	if err != nil {
		return Breadcrumb{}, err
	}
	return result, nil
}

func validateParent(ctx context.Context, q Querier, parent string) error {
	if parent == "" {
		return ErrInvalidParent
	}
	exists, err := q.Exists(ctx, parent)
	if err != nil {
		return err
	}
	if !exists {
		return ErrInvalidParent
	}
	return nil
}

func (service *Service) Move(ctx context.Context, identity authentication.VerifiedIdentity, currentRoot, newRoot string) error {
	if service.accessService == nil || service.repository == nil {
		return ErrServiceNotReady
	}
	current, err := ValidateTitle(currentRoot)
	if err != nil {
		return err
	}
	newParent, err := ValidateTitle(newRoot)
	if err != nil {
		return err
	}
	if current == newParent {
		return ErrSameRoot
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionEdit); err != nil {
		return err
	}
	return service.repository.Transaction(ctx, func(ctx context.Context, q Querier) error {
		total, err := q.Count(ctx)
		if err != nil {
			return err
		}
		if total == 0 {
			return ErrNoData
		}
		if exists, err := q.Exists(ctx, current); err != nil {
			return err
		} else if !exists {
			return ErrInvalidRoot
		}
		if exists, err := q.Exists(ctx, newParent); err != nil {
			return err
		} else if !exists {
			return ErrInvalidRoot
		}
		return q.Move(ctx, current, newParent)
	})
}

func (service *Service) Delete(ctx context.Context, identity authentication.VerifiedIdentity, pageTitle string) error {
	if service.accessService == nil || service.repository == nil {
		return ErrServiceNotReady
	}
	title, err := ValidateTitle(pageTitle)
	if err != nil {
		return err
	}
	if _, err := service.accessService.Authorize(ctx, identity, TableVariable, authorization.ActionDelete); err != nil {
		return err
	}
	return service.repository.Transaction(ctx, func(ctx context.Context, q Querier) error {
		if exists, err := q.Exists(ctx, title); err != nil {
			return err
		} else if !exists {
			return ErrNotFound
		}
		return q.Delete(ctx, title)
	})
}
