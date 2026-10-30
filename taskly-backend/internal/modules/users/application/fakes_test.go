package application

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"taskly/backend/internal/modules/users/domain"
	sharedapp "taskly/backend/internal/shared/application"
	sharedomain "taskly/backend/internal/shared/domain"
)

type fakeRepository struct {
	mu     sync.Mutex
	users  map[string]domain.User
	order  []string
	nextID int
	err    error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{users: map[string]domain.User{}}
}

func (r *fakeRepository) NextID() string {
	r.nextID++
	return "00000000-0000-4000-8000-" + strings.Repeat("0", 11) + string(rune('0'+r.nextID))
}

func (r *fakeRepository) Add(_ context.Context, user domain.User) error {
	if r.err != nil {
		return r.err
	}
	r.users[user.ID()] = user
	r.order = append(r.order, user.ID())
	return nil
}

func (r *fakeRepository) Update(_ context.Context, user domain.User) (bool, error) {
	if _, ok := r.users[user.ID()]; !ok {
		return false, nil
	}
	r.users[user.ID()] = user
	return true, nil
}

func (r *fakeRepository) FindByID(_ context.Context, id string) (domain.User, bool, error) {
	user, ok := r.users[id]
	return user, ok, nil
}

func (r *fakeRepository) FindByEmail(_ context.Context, email string) (domain.User, bool, error) {
	for _, id := range r.order {
		user, ok := r.users[id]
		if ok && strings.EqualFold(user.Email().Value(), email) {
			return user, true, nil
		}
	}
	return domain.User{}, false, nil
}

func (r *fakeRepository) ExistsByEmailOrUsername(_ context.Context, query domain.UniquenessQuery) (bool, error) {
	for id, user := range r.users {
		if id == query.ExcludeID {
			continue
		}
		if query.Email != "" && strings.EqualFold(user.Email().Value(), query.Email) {
			return true, nil
		}
		if query.Username != "" && strings.EqualFold(user.Username().Value(), query.Username) {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepository) Search(_ context.Context, filter domain.SearchFilter) ([]domain.User, error) {
	result := []domain.User{}
	for _, id := range r.order {
		user, ok := r.users[id]
		if !ok {
			continue
		}
		if filter.Username != "" && !strings.Contains(strings.ToLower(user.Username().Value()), strings.ToLower(filter.Username)) {
			continue
		}
		if filter.Email != "" && !strings.Contains(strings.ToLower(user.Email().Value()), strings.ToLower(filter.Email)) {
			continue
		}
		result = append(result, user)
	}
	return result, nil
}

func (r *fakeRepository) DeleteByID(_ context.Context, id string) (bool, error) {
	if _, ok := r.users[id]; !ok {
		return false, nil
	}
	delete(r.users, id)
	return true, nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(_ context.Context, plain string) (string, error) {
	return "hashed:" + plain, nil
}

func (fakeHasher) Compare(_ context.Context, plain, hash string) bool { return hash == "hashed:"+plain }

type fakeTokens struct{}

func (fakeTokens) Issue(userID string) (string, error) { return "token:" + userID, nil }

func (fakeTokens) Verify(token string) (string, error) {
	if !strings.HasPrefix(token, "token:") {
		return "", errors.New("invalid")
	}
	return strings.TrimPrefix(token, "token:"), nil
}

type fakeAvatarStorage struct {
	removed []string
}

func (s *fakeAvatarStorage) Remove(_ context.Context, avatarPath string) {
	s.removed = append(s.removed, avatarPath)
}

type fakeBus struct {
	published []sharedomain.Event
}

func (b *fakeBus) Publish(_ context.Context, event sharedomain.Event) {
	b.published = append(b.published, event)
}

func (b *fakeBus) Subscribe(string, sharedapp.EventHandler) {}

var fixedNow = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return fixedNow }
