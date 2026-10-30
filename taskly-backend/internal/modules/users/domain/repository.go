package domain

import "context"

type UniquenessQuery struct {
	Email     string
	Username  string
	ExcludeID string
}

type SearchFilter struct {
	Username string
	Email    string
}

type Repository interface {
	NextID() string
	Add(ctx context.Context, user User) error
	Update(ctx context.Context, user User) (bool, error)
	FindByID(ctx context.Context, id string) (User, bool, error)
	FindByEmail(ctx context.Context, email string) (User, bool, error)
	ExistsByEmailOrUsername(ctx context.Context, query UniquenessQuery) (bool, error)
	Search(ctx context.Context, filter SearchFilter) ([]User, error)
	DeleteByID(ctx context.Context, id string) (bool, error)
}
