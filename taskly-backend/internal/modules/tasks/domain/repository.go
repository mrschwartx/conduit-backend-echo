package domain

import "context"

type ListQuery struct {
	OwnerID  string
	Status   string
	OrderBy  string
	Page     int
	PageSize int
}

type Page struct {
	Tasks []Task
	Total int
}

type Repository interface {
	NextID() string
	Add(ctx context.Context, task Task) error
	Update(ctx context.Context, task Task) (bool, error)
	FindByID(ctx context.Context, id string) (Task, bool, error)
	FindByOwner(ctx context.Context, query ListQuery) (Page, error)
	DeleteByID(ctx context.Context, id string) (bool, error)
	DeleteByOwner(ctx context.Context, ownerID string) (int64, error)
}
