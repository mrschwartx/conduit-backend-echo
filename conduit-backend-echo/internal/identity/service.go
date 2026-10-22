package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"conduit/backend/internal/platform/database"
	"conduit/backend/internal/platform/httpx"
	"conduit/backend/internal/platform/validation"
)

type repository interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	Update(ctx context.Context, u *User) error
}

// TokenIssuer signs JWTs (implemented by auth.Service).
type TokenIssuer interface {
	Issue(userID int64, username string) (string, error)
}

// Service holds the identity business rules.
type Service struct {
	repo      repository
	tokens    TokenIssuer
	v         *validation.Validator
	cost      int
	dummyHash []byte // compared against when the email is unknown (timing equalisation)
	now       func() time.Time
}

// NewService creates the service. bcryptCost must be within bcrypt's range.
func NewService(repo repository, tokens TokenIssuer, v *validation.Validator, bcryptCost int) (*Service, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("conduit-dummy-password"), bcryptCost)
	if err != nil {
		return nil, err
	}
	return &Service{
		repo: repo, tokens: tokens, v: v, cost: bcryptCost, dummyHash: dummy,
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func (s *Service) result(u *User) (*AuthResult, error) {
	tok, err := s.tokens.Issue(u.ID, u.Username)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: u, Token: tok}, nil
}

// validatePassword: 8-72 characters (and at most 72 bytes, bcrypt's hard limit).
// Passwords are intentionally NOT trimmed.
func (s *Service) validatePassword(errs validation.Errors, pw string) {
	s.v.Var(errs, "password", pw, "required,min=8,max=72")
	if !errs.Has("password") && len(pw) > 72 {
		errs.Add("password", "is too long (maximum is 72 bytes)")
	}
}

func (s *Service) emailTaken(ctx context.Context, email string, selfID int64) (bool, error) {
	u, err := s.repo.GetByEmail(ctx, email)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return u.ID != selfID, nil
}

func (s *Service) usernameTaken(ctx context.Context, username string, selfID int64) (bool, error) {
	u, err := s.repo.GetByUsername(ctx, username)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return u.ID != selfID, nil
}

// Register creates an account (422 validation -> 409 conflicts -> create).
func (s *Service) Register(ctx context.Context, in NewUserInput) (*AuthResult, error) {
	username := strings.TrimSpace(in.Username)
	email := strings.ToLower(strings.TrimSpace(in.Email))

	errs := validation.Errors{}
	s.v.Var(errs, "username", username, "required,min=3,max=30,username")
	s.v.Var(errs, "email", email, "required,max=254,email")
	s.validatePassword(errs, in.Password)
	if len(errs) > 0 {
		return nil, httpx.Validation(errs)
	}

	dup := validation.Errors{}
	if taken, err := s.usernameTaken(ctx, username, 0); err != nil {
		return nil, err
	} else if taken {
		dup.Add("username", "has already been taken")
	}
	if taken, err := s.emailTaken(ctx, email, 0); err != nil {
		return nil, err
	} else if taken {
		dup.Add("email", "has already been taken")
	}
	if len(dup) > 0 {
		return nil, httpx.Conflict(dup)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), s.cost)
	if err != nil {
		return nil, err
	}
	now := s.now()
	u := &User{Username: username, Email: email, PasswordHash: string(hash), CreatedAt: now, UpdatedAt: now}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, mapConflict(err)
	}
	return s.result(u)
}

func mapConflict(err error) error {
	switch {
	case errors.Is(err, ErrDuplicateEmail):
		return httpx.Conflict(map[string][]string{"email": {"has already been taken"}})
	case errors.Is(err, ErrDuplicateUsername):
		return httpx.Conflict(map[string][]string{"username": {"has already been taken"}})
	}
	return err
}

// Login authenticates by email. Unknown email and wrong password are
// indistinguishable (same 401 and comparable timing).
func (s *Service) Login(ctx context.Context, in LoginInput) (*AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	errs := validation.Errors{}
	s.v.Var(errs, "email", email, "required")
	s.v.Var(errs, "password", in.Password, "required")
	if len(errs) > 0 {
		return nil, httpx.Validation(errs)
	}

	invalid := httpx.Unauthorized("credentials", "are invalid")
	u, err := s.repo.GetByEmail(ctx, email)
	if errors.Is(err, database.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(in.Password))
		return nil, invalid
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		return nil, invalid
	}
	return s.result(u)
}

func (s *Service) loadCurrent(ctx context.Context, userID int64) (*User, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if errors.Is(err, database.ErrNotFound) {
		return nil, httpx.Unauthorized("token", "is invalid")
	}
	return u, err
}

// Current returns the authenticated user with a fresh token.
func (s *Service) Current(ctx context.Context, userID int64) (*AuthResult, error) {
	u, err := s.loadCurrent(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.result(u)
}

// Update applies a partial update (absent = unchanged, null on bio/image = clear).
func (s *Service) Update(ctx context.Context, userID int64, in UpdateUserInput) (*AuthResult, error) {
	u, err := s.loadCurrent(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !in.Email.Set && !in.Username.Set && !in.Password.Set && !in.Bio.Set && !in.Image.Set {
		return nil, httpx.ValidationField("user", "must include at least one field")
	}

	errs := validation.Errors{}
	var newEmail, newUsername, newPassword *string

	if in.Email.Set {
		if in.Email.Null {
			errs.Add("email", "can't be blank")
		} else {
			v := strings.ToLower(strings.TrimSpace(in.Email.Value))
			s.v.Var(errs, "email", v, "required,max=254,email")
			if !errs.Has("email") {
				newEmail = &v
			}
		}
	}
	if in.Username.Set {
		if in.Username.Null {
			errs.Add("username", "can't be blank")
		} else {
			v := strings.TrimSpace(in.Username.Value)
			s.v.Var(errs, "username", v, "required,min=3,max=30,username")
			if !errs.Has("username") {
				newUsername = &v
			}
		}
	}
	if in.Password.Set {
		if in.Password.Null {
			errs.Add("password", "can't be blank")
		} else {
			pw := in.Password.Value
			s.validatePassword(errs, pw)
			if !errs.Has("password") {
				newPassword = &pw
			}
		}
	}

	var newBio, newImage *string
	clearBio, clearImage := false, false
	if in.Bio.Set {
		v := ""
		if !in.Bio.Null {
			v = strings.TrimSpace(in.Bio.Value)
		}
		if v == "" {
			clearBio = true
		} else {
			s.v.Var(errs, "bio", v, "max=300")
			newBio = &v
		}
	}
	if in.Image.Set {
		v := ""
		if !in.Image.Null {
			v = strings.TrimSpace(in.Image.Value)
		}
		if v == "" {
			clearImage = true
		} else {
			s.v.Var(errs, "image", v, "max=2048,httpsurl")
			newImage = &v
		}
	}
	if len(errs) > 0 {
		return nil, httpx.Validation(errs)
	}

	dup := validation.Errors{}
	if newEmail != nil {
		if taken, err := s.emailTaken(ctx, *newEmail, u.ID); err != nil {
			return nil, err
		} else if taken {
			dup.Add("email", "has already been taken")
		}
	}
	if newUsername != nil {
		if taken, err := s.usernameTaken(ctx, *newUsername, u.ID); err != nil {
			return nil, err
		} else if taken {
			dup.Add("username", "has already been taken")
		}
	}
	if len(dup) > 0 {
		return nil, httpx.Conflict(dup)
	}

	if newEmail != nil {
		u.Email = *newEmail
	}
	if newUsername != nil {
		u.Username = *newUsername
	}
	if newPassword != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*newPassword), s.cost)
		if err != nil {
			return nil, err
		}
		u.PasswordHash = string(hash)
	}
	if clearBio {
		u.Bio = nil
	} else if newBio != nil {
		u.Bio = newBio
	}
	if clearImage {
		u.Image = nil
	} else if newImage != nil {
		u.Image = newImage
	}
	u.UpdatedAt = s.now()

	if err := s.repo.Update(ctx, u); err != nil {
		return nil, mapConflict(err)
	}
	return s.result(u)
}
