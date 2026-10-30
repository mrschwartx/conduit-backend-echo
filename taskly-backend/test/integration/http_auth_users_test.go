//go:build integration

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"taskly/backend/test/support"
)

func startApp(t *testing.T) *support.App {
	t.Helper()
	database := support.StartTestDatabase(t)
	return support.StartApp(t, database.URL, nil)
}

func TestServiceBasics(t *testing.T) {
	app := startApp(t)

	t.Run("greets on the root path", func(t *testing.T) {
		response := app.NewClient().Get("/")
		expectStatus(t, response, 200)
		expectMessage(t, response, "Welcome to the Taskly API")
	})

	t.Run("responds with 404 for unknown routes", func(t *testing.T) {
		client := app.NewClient()
		for _, path := range []string{"/api/v1/unknown", "/images/nope.png", "/images/"} {
			response := client.Get(path)
			expectStatus(t, response, 404)
			expectMessage(t, response, "Not found")
		}
		response := client.Post("/api/v1/unknown")
		expectStatus(t, response, 404)
	})

	t.Run("responds with 400 for malformed JSON", func(t *testing.T) {
		response := app.NewClient().Post("/api/v1/auth/signin", support.RequestOptions{
			ContentType: "application/json",
			RawBody:     "{broken",
		})
		expectStatus(t, response, 400)
		expectMessage(t, response, "Malformed JSON body")
	})

	t.Run("responds with 413 for oversized JSON", func(t *testing.T) {
		response := app.NewClient().Post("/api/v1/auth/signin", support.RequestOptions{
			ContentType: "application/json",
			RawBody:     `{"email":"` + strings.Repeat("a", 200*1024) + `"}`,
		})
		expectStatus(t, response, 413)
		expectMessage(t, response, "Request body is too large")
	})

	t.Run("treats JSON arrays and missing content type as empty input", func(t *testing.T) {
		client := app.NewClient()
		array := client.Post("/api/v1/auth/signup", support.RequestOptions{ContentType: "application/json", RawBody: "[]"})
		expectStatus(t, array, 400)
		expectMessage(t, array, "Username is required")
		plain := client.Post("/api/v1/auth/signup", support.RequestOptions{RawBody: `{"username":"abc"}`})
		expectStatus(t, plain, 400)
		expectMessage(t, plain, "Username is required")
	})

	t.Run("does not advertise the framework", func(t *testing.T) {
		response := app.NewClient().Get("/")
		expectEqual(t, response.Header.Get("X-Powered-By"), "")
	})

	t.Run("allows the configured client origin with credentials", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodOptions, app.BaseURL+"/api/v1/auth/signin", nil)
		request.Header.Set("Origin", "http://localhost:3000")
		request.Header.Set("Access-Control-Request-Method", "POST")
		request.Header.Set("Access-Control-Request-Headers", "content-type")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		expectEqual(t, response.Header.Get("Access-Control-Allow-Origin"), "http://localhost:3000")
		expectEqual(t, response.Header.Get("Access-Control-Allow-Credentials"), "true")
	})
}

func TestSignUp(t *testing.T) {
	app := startApp(t)

	t.Run("creates a user and starts a session", func(t *testing.T) {
		client := app.NewClient()
		creds, response := support.RegisterUser(t, client)

		expectStatus(t, response, 200)
		expectEqual(t, response.String("username"), creds.Username)
		expectEqual(t, response.String("email"), creds.Email)
		expectEqual(t, response.String("avatar"), "/images/users/default-avatar.jpg")
		if !uuidPattern.MatchString(response.String("_id")) {
			t.Fatalf("unexpected id %q", response.String("_id"))
		}
		if _, exposed := response.Body["password"]; exposed {
			t.Fatal("password must not be exposed")
		}
		if _, exposed := response.Body["passwordHash"]; exposed {
			t.Fatal("password hash must not be exposed")
		}
		if !client.HasCookie("taskly_token") {
			t.Fatal("session cookie missing")
		}
	})

	t.Run("sets an http only cookie", func(t *testing.T) {
		_, response := support.RegisterUser(t, app.NewClient())
		var cookie string
		for _, value := range response.Header.Values("Set-Cookie") {
			if strings.HasPrefix(value, "taskly_token=") {
				cookie = value
			}
		}
		for _, attribute := range []string{"HttpOnly", "SameSite=Lax", "Max-Age=3600", "Path=/"} {
			if !strings.Contains(cookie, attribute) {
				t.Fatalf("cookie %q lacks %s", cookie, attribute)
			}
		}
	})

	t.Run("rejects duplicate emails and usernames with 422", func(t *testing.T) {
		creds, _ := support.RegisterUser(t, app.NewClient())

		sameEmail, sameEmailResponse := support.RegisterUser(t, app.NewClient(), func(c *support.Credentials) { c.Email = strings.ToUpper(creds.Email) })
		_ = sameEmail
		sameUsername, sameUsernameResponse := support.RegisterUser(t, app.NewClient(), func(c *support.Credentials) { c.Username = creds.Username })
		_ = sameUsername

		for _, response := range []support.Response{sameEmailResponse, sameUsernameResponse} {
			expectStatus(t, response, 422)
			expectMessage(t, response, "Email or Username is already registered.")
			if response.Body["status"] != float64(422) {
				t.Fatalf("status field = %v", response.Body["status"])
			}
		}
	})

	t.Run("rejects invalid payloads with 400", func(t *testing.T) {
		client := app.NewClient()
		payloads := []map[string]any{
			{},
			{"username": "ab", "email": "a@mail.com", "password": "Password123!"},
			{"username": "valid_user", "email": "nope", "password": "Password123!"},
			{"username": "valid_user", "email": "a@mail.com", "password": "short"},
			{"username": 12345, "email": "a@mail.com", "password": "Password123!"},
		}
		for _, payload := range payloads {
			expectStatus(t, client.Post("/api/v1/auth/signup", support.RequestOptions{JSON: payload}), 400)
		}
	})
}

func TestSignIn(t *testing.T) {
	app := startApp(t)

	t.Run("signs in with valid credentials", func(t *testing.T) {
		creds, id := support.MustRegisterUser(t, app.NewClient())
		client := app.NewClient()
		response := client.Post("/api/v1/auth/signin", support.RequestOptions{JSON: map[string]any{
			"email": strings.ToUpper(creds.Email), "password": creds.Password,
		}})
		expectStatus(t, response, 200)
		expectEqual(t, response.String("_id"), id)
		if !client.HasCookie("taskly_token") {
			t.Fatal("session cookie missing")
		}
	})

	t.Run("answers 404 for an unknown email", func(t *testing.T) {
		response := app.NewClient().Post("/api/v1/auth/signin", support.RequestOptions{JSON: map[string]any{
			"email": "ghost@mail.com", "password": "Password123!",
		}})
		expectStatus(t, response, 404)
		expectMessage(t, response, "User not found!")
	})

	t.Run("answers 401 for a wrong password", func(t *testing.T) {
		creds, _ := support.MustRegisterUser(t, app.NewClient())
		response := app.NewClient().Post("/api/v1/auth/signin", support.RequestOptions{JSON: map[string]any{
			"email": creds.Email, "password": "definitely-wrong",
		}})
		expectStatus(t, response, 401)
		expectMessage(t, response, "Wrong password!")
	})

	t.Run("rejects operator injection in credentials", func(t *testing.T) {
		client := app.NewClient()
		response := client.Post("/api/v1/auth/signin", support.RequestOptions{JSON: map[string]any{
			"email": map[string]any{"$ne": nil}, "password": "Password123!",
		}})
		expectStatus(t, response, 400)
		if client.HasCookie("taskly_token") {
			t.Fatal("no session expected")
		}
	})
}

func TestSignOutAndGuard(t *testing.T) {
	app := startApp(t)

	t.Run("clears the session cookie", func(t *testing.T) {
		client := app.NewClient()
		support.MustRegisterUser(t, client)
		if !client.HasCookie("taskly_token") {
			t.Fatal("cookie expected before sign out")
		}

		response := client.Get("/api/v1/auth/signout")
		expectStatus(t, response, 200)
		expectMessage(t, response, "Sign out successful")
		if client.HasCookie("taskly_token") {
			t.Fatal("cookie should be cleared")
		}
		expectStatus(t, client.Get("/api/v1/users"), 401)
	})

	t.Run("answers 401 without a cookie", func(t *testing.T) {
		response := app.NewClient().Get("/api/v1/users")
		expectStatus(t, response, 401)
		expectMessage(t, response, "Unauthorized: Token not provided")
	})

	t.Run("answers 403 for an invalid token", func(t *testing.T) {
		response := app.NewClient().Get("/api/v1/users", support.RequestOptions{
			Headers: map[string]string{"Cookie": "taskly_token=garbage"},
		})
		expectStatus(t, response, 403)
		expectMessage(t, response, "Forbidden: Invalid token")
	})

	t.Run("protects every users route", func(t *testing.T) {
		client := app.NewClient()
		responses := []support.Response{
			client.Get("/api/v1/users/" + missingID),
			client.Patch("/api/v1/users/update/"+missingID, support.RequestOptions{JSON: map[string]any{}}),
			client.Post("/api/v1/users/upload/" + missingID),
			client.Delete("/api/v1/users/delete/" + missingID),
		}
		for _, response := range responses {
			expectStatus(t, response, 401)
		}
	})
}

func TestListUsers(t *testing.T) {
	app := startApp(t)

	t.Run("lists and filters users without exposing passwords", func(t *testing.T) {
		client := app.NewClient()
		creds, _ := support.RegisterUser(t, client, func(c *support.Credentials) { c.Username = "searchable_person" })

		all := client.Get("/api/v1/users")
		filtered := client.Get("/api/v1/users?username=SEARCHABLE&email=" + url.QueryEscape(creds.Email))

		expectStatus(t, all, 200)
		for _, item := range all.List {
			if _, exposed := item.(map[string]any)["password"]; exposed {
				t.Fatal("password exposed")
			}
		}
		expectEqual(t, len(filtered.List), 1)
		expectEqual(t, filtered.List[0].(map[string]any)["username"].(string), "searchable_person")
	})

	t.Run("answers 404 when nothing matches", func(t *testing.T) {
		client := app.NewClient()
		support.MustRegisterUser(t, client)
		response := client.Get("/api/v1/users?username=no_such_user_xyz")
		expectStatus(t, response, 404)
		expectMessage(t, response, "No users found")
	})

	t.Run("rejects repeated filter parameters", func(t *testing.T) {
		client := app.NewClient()
		support.MustRegisterUser(t, client)
		expectStatus(t, client.Get("/api/v1/users?username=a&username=b"), 400)
	})

	t.Run("does not treat filters as patterns", func(t *testing.T) {
		client := app.NewClient()
		support.MustRegisterUser(t, client)
		expectStatus(t, client.Get("/api/v1/users?username=.*"), 404)
		expectStatus(t, client.Get("/api/v1/users?username=%25"), 404)
		expectStatus(t, client.Get("/api/v1/users?username=__"), 404)
	})
}

func TestGetUser(t *testing.T) {
	app := startApp(t)
	client := app.NewClient()
	_, id := support.MustRegisterUser(t, client)

	t.Run("returns the user", func(t *testing.T) {
		response := client.Get("/api/v1/users/" + id)
		expectStatus(t, response, 200)
		expectEqual(t, response.String("_id"), id)
	})

	t.Run("answers 400 for malformed ids and 404 for unknown ids", func(t *testing.T) {
		malformed := client.Get("/api/v1/users/not-an-id")
		unknown := client.Get("/api/v1/users/" + missingID)
		expectStatus(t, malformed, 400)
		expectMessage(t, malformed, "Invalid user ID format")
		expectStatus(t, unknown, 404)
		expectMessage(t, unknown, "User not found!")
	})
}

func TestUpdateUser(t *testing.T) {
	app := startApp(t)

	t.Run("updates the profile and lets the user sign in with the new password", func(t *testing.T) {
		user := signIn(t, app)
		username := "renamed_" + support.RandomCredentials().Username

		response := user.client.Patch("/api/v1/users/update/"+user.id, support.RequestOptions{JSON: map[string]any{
			"username": username, "password": "BrandNewPassword1",
		}})
		expectStatus(t, response, 200)
		expectEqual(t, response.String("username"), username)

		fresh := app.NewClient()
		oldLogin := fresh.Post("/api/v1/auth/signin", support.RequestOptions{JSON: map[string]any{
			"email": user.creds.Email, "password": user.creds.Password,
		}})
		newLogin := fresh.Post("/api/v1/auth/signin", support.RequestOptions{JSON: map[string]any{
			"email": user.creds.Email, "password": "BrandNewPassword1",
		}})
		expectStatus(t, oldLogin, 401)
		expectStatus(t, newLogin, 200)
	})

	t.Run("ignores the avatar field sent by clients", func(t *testing.T) {
		user := signIn(t, app)
		response := user.client.Patch("/api/v1/users/update/"+user.id, support.RequestOptions{JSON: map[string]any{
			"username": "kept_" + support.RandomCredentials().Username, "avatar": "default-avatar.jpg",
		}})
		expectStatus(t, response, 200)
		expectEqual(t, response.String("avatar"), "/images/users/default-avatar.jpg")
	})

	t.Run("rejects updating another account with 401", func(t *testing.T) {
		victim := signIn(t, app)
		attacker := signIn(t, app)
		response := attacker.client.Patch("/api/v1/users/update/"+victim.id, support.RequestOptions{JSON: map[string]any{
			"username": "hijacked_name",
		}})
		expectStatus(t, response, 401)
		expectMessage(t, response, "You can only update your own account")
	})

	t.Run("rejects colliding usernames with 422 and empty updates with 400", func(t *testing.T) {
		first := signIn(t, app)
		second := signIn(t, app)
		collision := second.client.Patch("/api/v1/users/update/"+second.id, support.RequestOptions{JSON: map[string]any{
			"username": first.creds.Username,
		}})
		empty := second.client.Patch("/api/v1/users/update/"+second.id, support.RequestOptions{JSON: map[string]any{}})
		expectStatus(t, collision, 422)
		expectStatus(t, empty, 400)
	})

	t.Run("accepts the current username and email unchanged", func(t *testing.T) {
		user := signIn(t, app)
		response := user.client.Patch("/api/v1/users/update/"+user.id, support.RequestOptions{JSON: map[string]any{
			"username": user.creds.Username, "email": user.creds.Email,
		}})
		expectStatus(t, response, 200)
	})
}

func TestUploadAvatar(t *testing.T) {
	app := startApp(t)

	t.Run("stores the avatar, serves it, and replaces the previous one", func(t *testing.T) {
		user := signIn(t, app)
		before := len(avatarFiles(t, app.PublicDir))

		first := user.client.Post("/api/v1/users/upload/"+user.id, support.CreateImageForm(support.ImageFormOptions{}))
		expectStatus(t, first, 200)
		firstPath := first.String("avatar")
		if !strings.HasPrefix(firstPath, "/images/users/avatar-") || !strings.HasSuffix(firstPath, ".png") {
			t.Fatalf("unexpected avatar path %q", firstPath)
		}
		expectEqual(t, len(avatarFiles(t, app.PublicDir)), before+1)

		served, err := http.Get(app.BaseURL + firstPath)
		if err != nil {
			t.Fatal(err)
		}
		served.Body.Close()
		expectEqual(t, served.StatusCode, 200)
		expectEqual(t, served.Header.Get("Content-Type"), "image/png")
		expectEqual(t, served.Header.Get("X-Content-Type-Options"), "nosniff")

		second := user.client.Post("/api/v1/users/upload/"+user.id, support.CreateImageForm(support.ImageFormOptions{
			Type: "image/jpeg", Filename: "next.jpg",
		}))
		expectStatus(t, second, 200)
		if !strings.HasSuffix(second.String("avatar"), ".jpg") {
			t.Fatalf("unexpected avatar path %q", second.String("avatar"))
		}
		expectEqual(t, len(avatarFiles(t, app.PublicDir)), before+1)

		gone, err := http.Get(app.BaseURL + firstPath)
		if err != nil {
			t.Fatal(err)
		}
		gone.Body.Close()
		expectEqual(t, gone.StatusCode, 404)
	})

	t.Run("rejects non image uploads", func(t *testing.T) {
		user := signIn(t, app)
		before := len(avatarFiles(t, app.PublicDir))
		response := user.client.Post("/api/v1/users/upload/"+user.id, support.CreateImageForm(support.ImageFormOptions{
			Type: "text/html", Filename: "xss.html",
		}))
		expectStatus(t, response, 400)
		if !strings.Contains(response.String("message"), "Only JPEG, PNG, WEBP and GIF") {
			t.Fatalf("unexpected message %q", response.String("message"))
		}
		expectEqual(t, len(avatarFiles(t, app.PublicDir)), before)
	})

	t.Run("rejects files above 2MB and leaves nothing on disk", func(t *testing.T) {
		user := signIn(t, app)
		before := len(avatarFiles(t, app.PublicDir))
		response := user.client.Post("/api/v1/users/upload/"+user.id, support.CreateImageForm(support.ImageFormOptions{
			Size: 2*1024*1024 + 1,
		}))
		expectStatus(t, response, 400)
		expectMessage(t, response, "File size exceeds 2MB")
		expectEqual(t, len(avatarFiles(t, app.PublicDir)), before)
	})

	t.Run("accepts a file of exactly 2MB", func(t *testing.T) {
		user := signIn(t, app)
		response := user.client.Post("/api/v1/users/upload/"+user.id, support.CreateImageForm(support.ImageFormOptions{
			Size: 2 * 1024 * 1024,
		}))
		expectStatus(t, response, 200)
	})

	t.Run("rejects requests without a file", func(t *testing.T) {
		user := signIn(t, app)
		empty := user.client.Post("/api/v1/users/upload/"+user.id, support.EmptyMultipartForm())
		expectStatus(t, empty, 400)
		expectMessage(t, empty, "No file uploaded")
		none := user.client.Post("/api/v1/users/upload/" + user.id)
		expectStatus(t, none, 400)
		expectMessage(t, none, "No file uploaded")
	})

	t.Run("rejects uploads for another account before writing anything", func(t *testing.T) {
		victim := signIn(t, app)
		attacker := signIn(t, app)
		before := len(avatarFiles(t, app.PublicDir))
		response := attacker.client.Post("/api/v1/users/upload/"+victim.id, support.CreateImageForm(support.ImageFormOptions{}))
		expectStatus(t, response, 401)
		expectEqual(t, len(avatarFiles(t, app.PublicDir)), before)
	})

	t.Run("rejects a differently named file field", func(t *testing.T) {
		user := signIn(t, app)
		before := len(avatarFiles(t, app.PublicDir))
		response := user.client.Post("/api/v1/users/upload/"+user.id, support.CreateImageForm(support.ImageFormOptions{FieldName: "photo"}))
		expectStatus(t, response, 400)
		expectEqual(t, len(avatarFiles(t, app.PublicDir)), before)
	})
}

func TestDeleteUser(t *testing.T) {
	app := startApp(t)

	t.Run("deletes the account, ends the session and removes the avatar", func(t *testing.T) {
		user := signIn(t, app)
		expectStatus(t, user.client.Post("/api/v1/users/upload/"+user.id, support.CreateImageForm(support.ImageFormOptions{})), 200)
		before := len(avatarFiles(t, app.PublicDir))

		response := user.client.Delete("/api/v1/users/delete/" + user.id)
		expectStatus(t, response, 200)
		expectMessage(t, response, "User has been deleted")
		if user.client.HasCookie("taskly_token") {
			t.Fatal("cookie should be cleared")
		}
		expectEqual(t, len(avatarFiles(t, app.PublicDir)), before-1)

		login := app.NewClient().Post("/api/v1/auth/signin", support.RequestOptions{JSON: map[string]any{
			"email": user.creds.Email, "password": user.creds.Password,
		}})
		expectStatus(t, login, 404)
	})

	t.Run("rejects deleting another account with 401", func(t *testing.T) {
		victim := signIn(t, app)
		attacker := signIn(t, app)
		response := attacker.client.Delete("/api/v1/users/delete/" + victim.id)
		expectStatus(t, response, 401)
		expectMessage(t, response, "You can only delete your own account")

		stillThere := app.NewClient().Post("/api/v1/auth/signin", support.RequestOptions{JSON: map[string]any{
			"email": victim.creds.Email, "password": victim.creds.Password,
		}})
		expectStatus(t, stillThere, 200)
	})
}

func TestTokenExpiryIsOneHour(t *testing.T) {
	app := startApp(t)
	_, response := support.RegisterUser(t, app.NewClient())
	for _, value := range response.Header.Values("Set-Cookie") {
		if !strings.HasPrefix(value, "taskly_token=") {
			continue
		}
		parsed := (&http.Response{Header: http.Header{"Set-Cookie": {value}}}).Cookies()[0]
		remaining := time.Until(parsed.Expires)
		if remaining < 59*time.Minute || remaining > 61*time.Minute {
			t.Fatalf("cookie expires in %v", remaining)
		}
	}
}
