package template

import (
	"context"
	"time"
)

// User represents a user in the system. Users are typically created via OAuth
// using the AuthService but users can also be created directly for testing.
type User struct {
	ID int `json:"id" db:"id,omitempty"`

	// User's preferred name & email.
	Name  string `json:"name" db:"name"`
	Email string `json:"email" db:"email"`

	// Timestamps for user creation & last update.
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`

	// List of associated OAuth authentication objects.
	Auths []*Auth `json:"auths" db:"-"`
}

// Validate returns an error if the user contains invalid fields.
// This only performs basic validation.
func (u *User) Validate() error {
	if u.Name == "" {
		return Errorf(EINVALID, "User name required.")
	}
	return nil
}

// AvatarURL returns a URL to the avatar image for the user.
// This loops over all auth providers to find the first available avatar.
// Currently only GitHub is supported. Returns blank string if no avatar URL available.
func (u *User) AvatarURL(size int) string {
	for _, auth := range u.Auths {
		if s := auth.AvatarURL(size); s != "" {
			return s
		}
	}
	return ""
}

// UserService represents a service for managing users.
type UserService interface {
	// Retrieves a user by ID along with their associated auth objects.
	// Returns ENOTFOUND if user does not exist.
	GetUserByID(ctx context.Context, id int) (*User, error)

	// Retrieves a list of users by filter. Also returns total count of matching
	// users which may differ from returned results if filter.Limit is specified.
	QueryUsers(ctx context.Context, query UserQuery) ([]*User, int, error)

	// Creates a new user. This is only used for testing since users are typically
	// created during the OAuth creation process in AuthService.CreateAuth().
	CreateUser(ctx context.Context, user *User) error

	// Updates a user object. Returns EUNAUTHORIZED if current user is not
	// the user that is being updated. Returns ENOTFOUND if user does not exist.
	UpdateUser(ctx context.Context, id int, update UserUpdate) (*User, error)

	// Permanently deletes a user and all owned snippets. Returns EUNAUTHORIZED
	// if current user is not the user being deleted. Returns ENOTFOUND if
	// user does not exist.
	DeleteUser(ctx context.Context, id int) error
}

// UserQuery represents a filter passed to UserService.QueryUsers().
type UserQuery struct {
	// Filtering fields.
	ID     *int    `json:"id"`
	Email  *string `json:"email"`

	// Restrict to subset of results.
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// UserQueryResult represents the result returned after querying for snippets using UserService.QueryUsers().
type UserQueryResult struct {
	Users []*User `json:"users"`
	Total int     `json:"total"`
}

// UserUpdate represents a set of fields to be updated via UserService.UpdateUser().
type UserUpdate struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}