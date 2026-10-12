package breadcrumbs

import (
	"context"
	"errors"
	"testing"

	"cch.local/backend/internal/modules/authorization"
	"cch.local/backend/internal/platform/authentication"
)

type repositoryStub struct {
	total    int64
	exists   map[string]bool
	nodes    map[string]Breadcrumb
	ancestrs []Ancestor
	created  []Breadcrumb
	moved    [][2]string
	deleted  []string
	err      error
}

func (repository *repositoryStub) Transaction(_ context.Context, fn func(context.Context, Querier) error) error {
	return fn(context.Background(), repository)
}
func (repository *repositoryStub) Count(context.Context) (int64, error) {
	return repository.total, repository.err
}
func (repository *repositoryStub) Exists(_ context.Context, title string) (bool, error) {
	if repository.exists == nil {
		return false, nil
	}
	return repository.exists[title], repository.err
}
func (repository *repositoryStub) Get(_ context.Context, title string) (Breadcrumb, error) {
	if repository.nodes == nil {
		return Breadcrumb{}, ErrNotFound
	}
	node, ok := repository.nodes[title]
	if !ok {
		return Breadcrumb{}, ErrNotFound
	}
	return node, repository.err
}
func (repository *repositoryStub) List(context.Context, ListQuery) (Page, error) {
	return Page{Data: []Breadcrumb{{PageTitle: "Home", Left: 1, Right: 2}}, Total: 1}, repository.err
}
func (repository *repositoryStub) Ancestors(context.Context, string) ([]Ancestor, error) {
	return repository.ancestrs, repository.err
}
func (repository *repositoryStub) Add(_ context.Context, title, url, _ string, left, right int) error {
	repository.created = append(repository.created, Breadcrumb{PageTitle: title, PageURL: url, Left: left, Right: right})
	return repository.err
}
func (repository *repositoryStub) Move(_ context.Context, current, newParent string) error {
	repository.moved = append(repository.moved, [2]string{current, newParent})
	return repository.err
}
func (repository *repositoryStub) Delete(_ context.Context, title string) error {
	repository.deleted = append(repository.deleted, title)
	return repository.err
}

type accessLoaderStub struct {
	access authorization.AccountAccess
	err    error
}

func (loader *accessLoaderStub) Load(_ context.Context, _ authentication.VerifiedIdentity, _ string) (authorization.AccountAccess, error) {
	return loader.access, loader.err
}

func newBreadcrumbService(repository *repositoryStub, mask authorization.Action) *Service {
	loader := &accessLoaderStub{access: authorization.AccountAccess{UserLevelIDs: []int64{29}, PermissionMask: int64(mask)}}
	return NewService(authorization.NewAccessService(loader), repository)
}

func TestServiceAddsBreadcrumbUnderRoot(t *testing.T) {
	repository := &repositoryStub{total: 1, exists: map[string]bool{"Home": true}, nodes: map[string]Breadcrumb{"Home": {PageTitle: "Home", PageURL: "/", Left: 1, Right: 2}}}
	service := newBreadcrumbService(repository, authorization.ActionAdd)
	created, err := service.Add(context.Background(), authentication.VerifiedIdentity{}, "Home", "Contact", "/contact")
	if err != nil {
		t.Fatal(err)
	}
	if created.PageTitle != "Contact" || created.Left != 2 || created.Right != 3 {
		t.Fatalf("unexpected created node: %#v", created)
	}
	if len(repository.created) != 1 {
		t.Fatalf("expected one insert, got %#v", repository.created)
	}
}

func TestServiceAddsRootOnEmptyTree(t *testing.T) {
	repository := &repositoryStub{}
	service := newBreadcrumbService(repository, authorization.ActionAdd)
	created, err := service.Add(context.Background(), authentication.VerifiedIdentity{}, "", "Home", "/")
	if err != nil {
		t.Fatal(err)
	}
	if created.PageTitle != "Home" || created.Left != 1 || created.Right != 2 {
		t.Fatalf("unexpected root node: %#v", created)
	}
}

func TestServiceRejectsInvalidInputAndMissingRoot(t *testing.T) {
	repository := &repositoryStub{}
	service := newBreadcrumbService(repository, authorization.ActionAdd)
	if _, err := service.Add(context.Background(), authentication.VerifiedIdentity{}, "", "", ""); err == nil {
		t.Fatal("empty title should be rejected")
	}
	if _, err := service.Add(context.Background(), authentication.VerifiedIdentity{}, "Home", "Child", "/child"); !errors.Is(err, ErrInvalidParent) {
		t.Fatalf("missing root error = %v, want ErrInvalidParent", err)
	}
}

func TestServiceMoveRequiresValidRoots(t *testing.T) {
	repository := &repositoryStub{total: 2, exists: map[string]bool{"Home": true, "About": true}}
	service := newBreadcrumbService(repository, authorization.ActionEdit)
	if err := service.Move(context.Background(), authentication.VerifiedIdentity{}, "Home", "Home"); !errors.Is(err, ErrSameRoot) {
		t.Fatalf("same-root error = %v, want ErrSameRoot", err)
	}
	if err := service.Move(context.Background(), authentication.VerifiedIdentity{}, "Home", "Missing"); !errors.Is(err, ErrInvalidRoot) {
		t.Fatalf("missing destination error = %v, want ErrInvalidRoot", err)
	}
}
