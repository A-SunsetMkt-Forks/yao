package user

import (
	"context"
	"fmt"
	"time"

	"github.com/yaoapp/gou/model"
	"github.com/yaoapp/kun/maps"
	"golang.org/x/crypto/bcrypt"
)

// User Basic Operations

// GetUser retrieves user information using the global user_id
func (u *DefaultUser) GetUser(ctx context.Context, userID string) (maps.MapStrAny, error) {
	m := model.Select(u.model)
	users, err := m.Get(model.QueryParam{
		Select: u.publicUserFields,
		Wheres: []model.QueryWhere{
			{Column: "user_id", Value: userID},
		},
		Limit: 1,
	})

	if err != nil {
		return nil, fmt.Errorf(ErrFailedToGetUser, err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf(ErrUserNotFound)
	}

	return users[0], nil
}

// GetUserByPreferredUsername retrieves user by preferred_username (OIDC standard)
func (u *DefaultUser) GetUserByPreferredUsername(ctx context.Context, preferredUsername string) (maps.MapStrAny, error) {
	m := model.Select(u.model)
	users, err := m.Get(model.QueryParam{
		Select: u.publicUserFields,
		Wheres: []model.QueryWhere{
			{Column: "preferred_username", Value: preferredUsername},
		},
		Limit: 1,
	})

	if err != nil {
		return nil, fmt.Errorf(ErrFailedToGetUser, err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf(ErrUserNotFound)
	}

	return users[0], nil
}

// GetUserByEmail retrieves user by email address
func (u *DefaultUser) GetUserByEmail(ctx context.Context, email string) (maps.MapStrAny, error) {
	m := model.Select(u.model)
	users, err := m.Get(model.QueryParam{
		Select: u.publicUserFields,
		Wheres: []model.QueryWhere{
			{Column: "email", Value: email},
		},
		Limit: 1,
	})

	if err != nil {
		return nil, fmt.Errorf(ErrFailedToGetUser, err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf(ErrUserNotFound)
	}

	return users[0], nil
}

// GetUserForAuth retrieves user information for authentication purposes (internal use only)
func (u *DefaultUser) GetUserForAuth(ctx context.Context, identifier string, identifierType string) (maps.MapStrAny, error) {
	m := model.Select(u.model)

	var column string
	switch identifierType {
	case "user_id":
		column = "user_id"
	case "preferred_username":
		column = "preferred_username"
	case "email":
		column = "email"
	case "phone_number":
		column = "phone_number"
	default:
		return nil, fmt.Errorf(ErrInvalidIdentifierType, identifierType)
	}

	users, err := m.Get(model.QueryParam{
		Select: u.authUserFields,
		Wheres: []model.QueryWhere{
			{Column: column, Value: identifier},
		},
		Limit: 1,
	})

	if err != nil {
		return nil, fmt.Errorf(ErrFailedToGetUser, err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf(ErrUserNotFound)
	}

	return users[0], nil
}

// VerifyPassword verifies password against password hash (no database query needed)
func (u *DefaultUser) VerifyPassword(ctx context.Context, password string, passwordHash string) (bool, error) {
	if passwordHash == "" {
		return false, fmt.Errorf(ErrNoPasswordHash)
	}

	// Verify password using bcrypt (copied from yao/helper/password.go logic)
	err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		return false, nil // Invalid password, but no error (return false)
	}

	return true, nil
}

// UpdatePassword updates user password (requires current password verification)
func (u *DefaultUser) UpdatePassword(ctx context.Context, userID string, newPassword string) error {
	updateData := maps.MapStrAny{
		"password_hash":       newPassword, // Yao will auto-hash
		"password_changed_at": time.Now(),
	}

	m := model.Select(u.model)
	affected, err := m.UpdateWhere(model.QueryParam{
		Wheres: []model.QueryWhere{
			{Column: "user_id", Value: userID},
		},
		Limit: 1, // Safety: ensure only one record is updated
	}, updateData)

	if err != nil {
		return fmt.Errorf(ErrFailedToUpdateUser, err)
	}

	if affected == 0 {
		return fmt.Errorf(ErrUserNotFound)
	}

	return nil
}

// ResetPassword generates and sets a new random password (admin/recovery operation)
func (u *DefaultUser) ResetPassword(ctx context.Context, userID string) (string, error) {
	// Generate a random password
	randomPassword, err := generateRandomPassword(12) // 12 characters
	if err != nil {
		return "", fmt.Errorf(ErrFailedToGeneratePassword, err)
	}

	updateData := maps.MapStrAny{
		"password_hash":       randomPassword, // Yao will auto-hash
		"password_changed_at": time.Now(),
	}

	m := model.Select(u.model)
	affected, err := m.UpdateWhere(model.QueryParam{
		Wheres: []model.QueryWhere{
			{Column: "user_id", Value: userID},
		},
		Limit: 1, // Safety: ensure only one record is updated
	}, updateData)

	if err != nil {
		return "", fmt.Errorf(ErrFailedToUpdateUser, err)
	}

	if affected == 0 {
		return "", fmt.Errorf(ErrUserNotFound)
	}

	return randomPassword, nil
}

// CreateUser creates a new user with OIDC standard fields
func (u *DefaultUser) CreateUser(ctx context.Context, userData maps.MapStrAny) (interface{}, error) {
	// Auto-generate user_id if not provided
	if _, exists := userData["user_id"]; !exists {
		userID, err := u.GenerateUserID(ctx, true) // Force safe mode to ensure uniqueness
		if err != nil {
			return nil, fmt.Errorf(ErrFailedToGenerateUserID, err)
		}
		userData["user_id"] = userID
	}

	// Yao Model will auto-hash password if provided as password_hash field
	if password, ok := userData["password"].(string); ok && password != "" {
		userData["password_hash"] = password // Let Yao handle the hashing
		delete(userData, "password")         // Remove plain password key
	}

	// Set default status if not provided
	if _, exists := userData["status"]; !exists {
		userData["status"] = "pending"
	}

	m := model.Select(u.model)
	id, err := m.Create(userData)
	if err != nil {
		return nil, fmt.Errorf(ErrFailedToCreateUser, err)
	}

	return id, nil
}

// UpdateUser updates user information (excludes sensitive fields like password, MFA)
func (u *DefaultUser) UpdateUser(ctx context.Context, userID string, userData maps.MapStrAny) error {
	// Remove sensitive fields that should use dedicated methods
	sensitiveFields := []string{
		"password", "password_hash", "password_changed_at",
		"mfa_secret", "mfa_recovery_hash", "mfa_enabled", "mfa_enabled_at",
	}

	for _, field := range sensitiveFields {
		delete(userData, field)
	}

	// Skip update if no valid fields remain
	if len(userData) == 0 {
		return nil
	}

	m := model.Select(u.model)
	affected, err := m.UpdateWhere(model.QueryParam{
		Wheres: []model.QueryWhere{
			{Column: "user_id", Value: userID},
		},
		Limit: 1, // Safety: ensure only one record is updated
	}, userData)

	if err != nil {
		return fmt.Errorf(ErrFailedToUpdateUser, err)
	}

	if affected == 0 {
		return fmt.Errorf(ErrUserNotFound)
	}

	return nil
}

// DeleteUser soft deletes a user account
func (u *DefaultUser) DeleteUser(ctx context.Context, userID string) error {
	m := model.Select(u.model)
	affected, err := m.DeleteWhere(model.QueryParam{
		Wheres: []model.QueryWhere{
			{Column: "user_id", Value: userID},
		},
		Limit: 1, // Safety: ensure only one record is deleted
	})

	if err != nil {
		return fmt.Errorf(ErrFailedToDeleteUser, err)
	}

	if affected == 0 {
		return fmt.Errorf(ErrUserNotFound)
	}

	return nil
}

// UpdateUserLastLogin updates the user's last login timestamp
func (u *DefaultUser) UpdateUserLastLogin(ctx context.Context, userID string) error {
	updateData := maps.MapStrAny{
		"last_login_at": time.Now(),
	}

	return u.UpdateUser(ctx, userID, updateData)
}

// UpdateUserStatus updates user account status (active, disabled, suspended, etc.)
func (u *DefaultUser) UpdateUserStatus(ctx context.Context, userID string, status string) error {
	updateData := maps.MapStrAny{
		"status": status,
	}

	return u.UpdateUser(ctx, userID, updateData)
}
