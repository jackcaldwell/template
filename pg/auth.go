package pg

import (
	"context"
	"fmt"
	"template"
)

// AuthService represents a service for managing OAuth authentication.
type AuthService struct {
	db *DB
}

// NewAuthService returns a new instance of AuthService attached to DB.
func NewAuthService(db *DB) template.AuthService {
	return &AuthService{db: db}
}

func (s AuthService) GetAuthByID(ctx context.Context, id int) (*template.Auth, error) {
	panic("implement me")
}

func (s AuthService) QueryAuths(ctx context.Context, filter template.AuthQuery) ([]*template.Auth, int, error) {
	panic("implement me")
}

// CreateAuth Creates a new authentication object. If a User is attached to auth,
// then the auth object is linked to an existing user. Otherwise a new user
// object is created.
//
// On success, the auth.ID is set to the new authentication ID.
func (s AuthService) CreateAuth(ctx context.Context, auth *template.Auth) error {
	tx, err := s.db.beginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check to see if the the auth already exists for the given source.
	if _, err := tx.findAuthBySourceID(ctx, auth.Source, auth.SourceID); err == nil {
		// If an auth already exists for the source user, update it with the new tokens and expiry.

	} else if template.ErrorCode(err) != template.ENOTFOUND {
		return fmt.Errorf("find auth by source id failed: %w", err)
	}

	// Check if auth has a new user object passed in. It is considered "new" if
	// the caller doesn't know the database ID for the user.
	if auth.UserID == 0 && auth.User != nil {
		// Look up the user by email address. If no user can be found then
		// create a new user with the auth.User object passed in.
		user, err := tx.findUserByEmail(ctx, auth.User.Email)
		if err != nil {
			if template.ErrorCode(err) != template.ENOTFOUND {
				// Something went wrong while finding the user
				return fmt.Errorf("find user by email failed: %w", err)
			}
			// User does not exist so should be created
			if err := tx.insert(ctx, auth.User, "users"); err != nil {
				return fmt.Errorf("cannot create user: %w", err)
			}
		} else {
			// User found in database so auth.User should be updated.
			auth.User = user
		}
		// Assign the created/found user ID back to the auth object.
		auth.UserID = auth.User.ID
	}

	// Create new auth object and attach associated user.
	if err := tx.insert(ctx, auth, "auths"); err != nil {
		return err
	}

	return tx.Commit()
}

func (s AuthService) DeleteAuth(ctx context.Context, id int) error {
	panic("implement me")
}

func (tx *Tx) findAuthBySourceID(ctx context.Context, source string, sourceID string) (*template.Auth, error) {
	var auth template.Auth

	if err := tx.Get(&auth, "SELECT * FROM auths WHERE source = $1 AND source_id = $2", source, sourceID); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, &template.Error{Code: template.ENOTFOUND, Message: "Auth not found."}
		}
		return nil, err
	}

	return &auth, nil
}
