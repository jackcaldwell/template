package pg

import (
	"context"
	"template"
)

// UserService represents a service for managing users.
type UserService struct {
	db *DB
}

func NewUserService(db *DB) template.UserService {
	return &UserService{db: db}
}

// GetUserByID retrieves a user by ID and their associated auth objects.
// Returns ENOTFOUND if user does not exist.
func (s UserService) GetUserByID(ctx context.Context, id int) (*template.User, error) {
	tx, err := s.db.beginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Fetch user and their associated auths
	user, err := tx.findUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.attachUserAuths(ctx, user); err != nil {
		return user, err
	}

	return user, nil
}

func (s UserService) QueryUsers(ctx context.Context, query template.UserQuery) ([]*template.User, int, error) {
	panic("implement me")
}

func (s UserService) CreateUser(ctx context.Context, user *template.User) error {
	panic("implement me")
}

func (s UserService) UpdateUser(ctx context.Context, id int, update template.UserUpdate) (*template.User, error) {
	panic("implement me")
}

func (s UserService) DeleteUser(ctx context.Context, id int) error {
	panic("implement me")
}

// findUserByEmail is a helper function to retrieve a user with a given email address.
// Returns ENOTFOUND if user does not exist.
func (tx *Tx) findUserByEmail(_ context.Context, email string) (*template.User, error) {
	var user template.User

	if err := tx.Get(&user, "SELECT * FROM users WHERE email = $1", email); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, &template.Error{Code: template.ENOTFOUND, Message: "User not found."}
		}
		return nil, err
	}

	return &user, nil
}

// findUserByID is a helper function to fetch a user by ID.
// Returns ENOTFOUND if user does not exist.
func (tx *Tx) findUserByID(ctx context.Context, id int) (*template.User, error) {
	var user template.User

	if err := tx.Get(&user, "SELECT * FROM users WHERE id = $1", id); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, &template.Error{Code: template.ENOTFOUND, Message: "User not found."}
		}
		return nil, err
	}

	if err := tx.attachUserAuths(ctx, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// attachUserAuths attaches auth objects associated with the user.
func (tx *Tx) attachUserAuths(ctx context.Context, user *template.User) (err error) {
	auths := make([]*template.Auth, 0)
	if err := tx.Select(&auths, "SELECT * FROM auths WHERE user_id = $1", user.ID); err != nil {
		return err
	}
	user.Auths = auths

	return nil
}