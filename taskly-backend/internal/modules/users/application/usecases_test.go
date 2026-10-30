package application

import (
	"context"
	"strings"
	"testing"

	sharedomain "taskly/backend/internal/shared/domain"
)

const (
	ownerID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	otherID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

type fixture struct {
	repo     *fakeRepository
	avatars  *fakeAvatarStorage
	bus      *fakeBus
	register *RegisterUser
	auth     *AuthenticateUser
	update   *UpdateUser
	delete   *DeleteUser
	avatar   *ChangeUserAvatar
	get      *GetUser
	list     *ListUsers
}

func newFixture() *fixture {
	repo := newFakeRepository()
	avatars := &fakeAvatarStorage{}
	bus := &fakeBus{}
	f := &fixture{
		repo:     repo,
		avatars:  avatars,
		bus:      bus,
		register: NewRegisterUser(repo, fakeHasher{}, fakeTokens{}),
		auth:     NewAuthenticateUser(repo, fakeHasher{}, fakeTokens{}),
		update:   NewUpdateUser(repo, fakeHasher{}),
		delete:   NewDeleteUser(repo, avatars, bus),
		avatar:   NewChangeUserAvatar(repo, avatars),
		get:      NewGetUser(repo),
		list:     NewListUsers(repo),
	}
	f.register.now = fixedClock
	f.update.now = fixedClock
	f.delete.now = fixedClock
	f.avatar.now = fixedClock
	return f
}

func (f *fixture) signUp(t *testing.T, username, email string) AuthResult {
	t.Helper()
	result, err := f.register.Execute(context.Background(), RegisterUserInput{
		Username: sharedomain.RawString(username),
		Email:    sharedomain.RawString(email),
		Password: sharedomain.RawString("Password123!"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func expectKind(t *testing.T, err error, kind sharedomain.ErrorKind, message string) {
	t.Helper()
	if !sharedomain.IsKind(err, kind) {
		t.Fatalf("expected kind %s, got %v", kind, err)
	}
	if message != "" && err.Error() != message {
		t.Fatalf("message = %q, want %q", err.Error(), message)
	}
}

func TestRegisterUser(t *testing.T) {
	f := newFixture()
	result := f.signUp(t, "  john_doe  ", "John@Mail.com")

	if result.User.Username != "john_doe" || result.User.Avatar != "/images/users/default-avatar.jpg" {
		t.Fatalf("unexpected user %+v", result.User)
	}
	if result.Token != "token:"+result.User.ID {
		t.Fatalf("unexpected token %q", result.Token)
	}
	if result.User.CreatedAt != "2026-05-01T12:00:00.000Z" {
		t.Fatalf("unexpected timestamp %q", result.User.CreatedAt)
	}
	stored := f.repo.users[result.User.ID]
	if stored.PasswordHash() != "hashed:Password123!" {
		t.Fatal("password must be stored hashed")
	}

	_, err := f.register.Execute(context.Background(), RegisterUserInput{
		Username: sharedomain.RawString("someone_else"),
		Email:    sharedomain.RawString("JOHN@mail.com"),
		Password: sharedomain.RawString("Password123!"),
	})
	expectKind(t, err, sharedomain.KindAlreadyExists, "Email or Username is already registered.")

	_, err = f.register.Execute(context.Background(), RegisterUserInput{Username: sharedomain.RawString("x")})
	expectKind(t, err, sharedomain.KindValidation, "Username must be between 3 and 50 characters")
}

func TestAuthenticateUser(t *testing.T) {
	f := newFixture()
	registered := f.signUp(t, "john_doe", "john@mail.com")
	ctx := context.Background()

	result, err := f.auth.Execute(ctx, AuthenticateUserInput{
		Email: sharedomain.RawString("  JOHN@mail.com "), Password: sharedomain.RawString("Password123!"),
	})
	if err != nil || result.User.ID != registered.User.ID {
		t.Fatalf("sign in failed: %v", err)
	}

	_, err = f.auth.Execute(ctx, AuthenticateUserInput{Email: sharedomain.RawString("john@mail.com"), Password: sharedomain.RawString("nope")})
	expectKind(t, err, sharedomain.KindAuthentication, "Wrong password!")

	_, err = f.auth.Execute(ctx, AuthenticateUserInput{Email: sharedomain.RawString("ghost@mail.com"), Password: sharedomain.RawString("x")})
	expectKind(t, err, sharedomain.KindNotFound, "User not found!")

	_, err = f.auth.Execute(ctx, AuthenticateUserInput{Email: sharedomain.RawOf(map[string]any{"$ne": nil}), Password: sharedomain.RawString("x")})
	expectKind(t, err, sharedomain.KindValidation, "Email and password are required")

	_, err = f.auth.Execute(ctx, AuthenticateUserInput{Email: sharedomain.RawString("john@mail.com")})
	expectKind(t, err, sharedomain.KindValidation, "Email and password are required")
}

func TestVerifyAccessToken(t *testing.T) {
	verify := NewVerifyAccessToken(fakeTokens{})
	if id, err := verify.Execute("token:abc"); err != nil || id != "abc" {
		t.Fatalf("got %q %v", id, err)
	}
	_, err := verify.Execute("")
	expectKind(t, err, sharedomain.KindAuthentication, "Unauthorized: Token not provided")
	_, err = verify.Execute("garbage")
	expectKind(t, err, sharedomain.KindForbidden, "Forbidden: Invalid token")
}

func TestGetAndListUsers(t *testing.T) {
	f := newFixture()
	first := f.signUp(t, "alice_one", "alice@mail.com")
	f.signUp(t, "bob_two", "bob@mail.com")
	ctx := context.Background()

	found, err := f.get.Execute(ctx, strings.ToUpper(first.User.ID))
	if err != nil || found.ID != first.User.ID {
		t.Fatalf("get failed: %v", err)
	}
	_, err = f.get.Execute(ctx, "bad")
	expectKind(t, err, sharedomain.KindValidation, "Invalid user ID format")
	_, err = f.get.Execute(ctx, otherID)
	expectKind(t, err, sharedomain.KindNotFound, "User not found!")

	all, err := f.list.Execute(ctx, ListUsersInput{})
	if err != nil || len(all) != 2 {
		t.Fatalf("list failed: %v %d", err, len(all))
	}
	filtered, _ := f.list.Execute(ctx, ListUsersInput{Username: []string{"ALICE"}})
	if len(filtered) != 1 {
		t.Fatalf("expected one match, got %d", len(filtered))
	}
	_, err = f.list.Execute(ctx, ListUsersInput{Username: []string{"zzz"}})
	expectKind(t, err, sharedomain.KindNotFound, "No users found")
	_, err = f.list.Execute(ctx, ListUsersInput{Email: []string{"a", "b"}})
	expectKind(t, err, sharedomain.KindValidation, "Email filter must be a string")
}

func TestUpdateUser(t *testing.T) {
	f := newFixture()
	me := f.signUp(t, "john_doe", "john@mail.com")
	other := f.signUp(t, "jane_doe", "jane@mail.com")
	ctx := context.Background()

	updated, err := f.update.Execute(ctx, UpdateUserInput{
		RequesterID: me.User.ID, UserID: strings.ToUpper(me.User.ID),
		Changes: UserChanges{Username: sharedomain.RawString("johnny"), Password: sharedomain.RawString("NewPassword1")},
	})
	if err != nil || updated.Username != "johnny" {
		t.Fatalf("update failed: %v", err)
	}
	if f.repo.users[me.User.ID].PasswordHash() != "hashed:NewPassword1" {
		t.Fatal("password was not rehashed")
	}

	_, err = f.update.Execute(ctx, UpdateUserInput{RequesterID: me.User.ID, UserID: other.User.ID, Changes: UserChanges{Username: sharedomain.RawString("hijack")}})
	expectKind(t, err, sharedomain.KindAuthentication, "You can only update your own account")

	_, err = f.update.Execute(ctx, UpdateUserInput{RequesterID: me.User.ID, UserID: me.User.ID})
	expectKind(t, err, sharedomain.KindValidation, "No updatable fields provided")

	_, err = f.update.Execute(ctx, UpdateUserInput{RequesterID: me.User.ID, UserID: me.User.ID, Changes: UserChanges{Email: sharedomain.RawString("JANE@mail.com")}})
	expectKind(t, err, sharedomain.KindAlreadyExists, "Email or Username is already registered.")

	_, err = f.update.Execute(ctx, UpdateUserInput{RequesterID: me.User.ID, UserID: me.User.ID, Changes: UserChanges{Username: sharedomain.RawOf(nil)}})
	expectKind(t, err, sharedomain.KindValidation, "Username is required")

	_, err = f.update.Execute(ctx, UpdateUserInput{RequesterID: me.User.ID, UserID: me.User.ID, Changes: UserChanges{Username: sharedomain.RawString("johnny"), Email: sharedomain.RawString("john@mail.com")}})
	if err != nil {
		t.Fatalf("keeping own username and email must be allowed: %v", err)
	}

	_, err = f.update.Execute(ctx, UpdateUserInput{RequesterID: otherID, UserID: otherID, Changes: UserChanges{Username: sharedomain.RawString("ghost")}})
	expectKind(t, err, sharedomain.KindNotFound, "User not found!")
}

func TestDeleteUser(t *testing.T) {
	f := newFixture()
	me := f.signUp(t, "john_doe", "john@mail.com")
	ctx := context.Background()
	stored := f.repo.users[me.User.ID]
	stored.ChangeAvatar("/images/users/avatar-1-2.png", fixedNow)
	f.repo.users[me.User.ID] = stored

	err := f.delete.Execute(ctx, DeleteUserInput{RequesterID: otherID, UserID: me.User.ID})
	expectKind(t, err, sharedomain.KindAuthentication, "You can only delete your own account")
	if len(f.repo.users) != 1 {
		t.Fatal("user must survive a rejected delete")
	}

	if err := f.delete.Execute(ctx, DeleteUserInput{RequesterID: me.User.ID, UserID: me.User.ID}); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.users) != 0 {
		t.Fatal("user must be deleted")
	}
	if len(f.avatars.removed) != 1 || f.avatars.removed[0] != "/images/users/avatar-1-2.png" {
		t.Fatalf("avatar not removed: %v", f.avatars.removed)
	}
	if len(f.bus.published) != 1 {
		t.Fatalf("expected one event, got %d", len(f.bus.published))
	}
	event := f.bus.published[0]
	payload, ok := event.Payload.(sharedomain.UserDeleted)
	if event.Name != sharedomain.UserDeletedEvent || !ok || payload.UserID != me.User.ID {
		t.Fatalf("unexpected event %+v", event)
	}

	err = f.delete.Execute(ctx, DeleteUserInput{RequesterID: me.User.ID, UserID: me.User.ID})
	expectKind(t, err, sharedomain.KindNotFound, "User not found!")
}

func TestChangeUserAvatar(t *testing.T) {
	f := newFixture()
	me := f.signUp(t, "john_doe", "john@mail.com")
	ctx := context.Background()

	first, err := f.avatar.Execute(ctx, ChangeUserAvatarInput{RequesterID: me.User.ID, UserID: me.User.ID, AvatarPath: "/images/users/avatar-1-1.png"})
	if err != nil || first.Avatar != "/images/users/avatar-1-1.png" {
		t.Fatalf("avatar change failed: %v", err)
	}
	if len(f.avatars.removed) != 1 || f.avatars.removed[0] != "/images/users/default-avatar.jpg" {
		t.Fatalf("previous avatar should be handed to storage: %v", f.avatars.removed)
	}

	f.avatars.removed = nil
	_, err = f.avatar.Execute(ctx, ChangeUserAvatarInput{RequesterID: me.User.ID, UserID: me.User.ID, AvatarPath: "/images/users/avatar-2-2.png"})
	if err != nil || f.avatars.removed[0] != "/images/users/avatar-1-1.png" {
		t.Fatalf("replacement failed: %v %v", err, f.avatars.removed)
	}
}

func TestChangeUserAvatarCleansUpTheNewFileOnFailure(t *testing.T) {
	f := newFixture()
	me := f.signUp(t, "john_doe", "john@mail.com")
	ctx := context.Background()
	upload := "/images/users/avatar-9-9.png"

	cases := []struct {
		name  string
		input ChangeUserAvatarInput
		kind  sharedomain.ErrorKind
	}{
		{"foreign account", ChangeUserAvatarInput{RequesterID: otherID, UserID: me.User.ID, AvatarPath: upload}, sharedomain.KindAuthentication},
		{"malformed id", ChangeUserAvatarInput{RequesterID: "bad", UserID: "bad", AvatarPath: upload}, sharedomain.KindValidation},
		{"unknown user", ChangeUserAvatarInput{RequesterID: otherID, UserID: otherID, AvatarPath: upload}, sharedomain.KindNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f.avatars.removed = nil
			_, err := f.avatar.Execute(ctx, tc.input)
			expectKind(t, err, tc.kind, "")
			if len(f.avatars.removed) != 1 || f.avatars.removed[0] != upload {
				t.Fatalf("upload must be removed, got %v", f.avatars.removed)
			}
		})
	}

	f.avatars.removed = nil
	_, err := f.avatar.Execute(ctx, ChangeUserAvatarInput{RequesterID: me.User.ID, UserID: me.User.ID})
	expectKind(t, err, sharedomain.KindValidation, "No file uploaded")
	if len(f.avatars.removed) != 0 {
		t.Fatal("nothing to clean up without an upload")
	}
}
