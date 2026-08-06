package services

import (
	"errors"

	"github.com/google/uuid"
	"github.com/tldr-it-stepankutaj/openvpn-mng/internal/database"
	"github.com/tldr-it-stepankutaj/openvpn-mng/internal/dto"
	"github.com/tldr-it-stepankutaj/openvpn-mng/internal/models"
	"gorm.io/gorm"
)

var (
	ErrUserExists = errors.New("user already exists")
)

// UserService provides user management services
type UserService struct{}

// NewUserService creates a new user service
func NewUserService() *UserService {
	return &UserService{}
}

// Create creates a new user
func (s *UserService) Create(req *dto.CreateUserRequest, createdBy uuid.UUID) (*models.User, error) {
	// Active users must keep unique usernames and email addresses.
	var existing models.User
	err := database.GetDB().Where("username = ? OR email = ?", req.Username, req.Email).First(&existing).Error
	if err == nil {
		return nil, ErrUserExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Hash password
	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// Default is_active to true if not specified
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	// Users are soft deleted. Their database unique indexes still reserve the
	// username and email, so restore a matching deleted account rather than
	// attempting an insert that would fail with a raw database constraint error.
	var deletedUsers []models.User
	if err := database.GetDB().Unscoped().
		Where("username = ? OR email = ?", req.Username, req.Email).
		Find(&deletedUsers).Error; err != nil {
		return nil, err
	}
	if len(deletedUsers) > 1 {
		// Username and email point to different historical accounts. Do not
		// silently merge them into one account.
		return nil, ErrUserExists
	}
	if len(deletedUsers) == 1 {
		user := &deletedUsers[0]
		updates := map[string]interface{}{
			"username":    req.Username,
			"password":    hashedPassword,
			"manager_id":  req.ManagerID,
			"first_name":  req.FirstName,
			"middle_name": req.MiddleName,
			"last_name":   req.LastName,
			"email":       req.Email,
			"telephone":   req.Telephone,
			"role":        req.Role,
			"is_active":   isActive,
			"valid_from":  req.ValidFrom.ToTimePtr(),
			"valid_to":    req.ValidTo.ToTimePtr(),
			"vpn_ip":      req.VpnIP,
			"updated_by":  createdBy,
			"deleted_at":  nil,
		}
		if err := database.GetDB().Unscoped().Model(user).Updates(updates).Error; err != nil {
			return nil, err
		}
		if err := database.GetDB().First(user, "id = ?", user.ID).Error; err != nil {
			return nil, err
		}
		return user, nil
	}

	user := &models.User{
		Username:   req.Username,
		Password:   hashedPassword,
		ManagerID:  req.ManagerID,
		FirstName:  req.FirstName,
		MiddleName: req.MiddleName,
		LastName:   req.LastName,
		Email:      req.Email,
		Telephone:  req.Telephone,
		Role:       req.Role,
		IsActive:   isActive,
		ValidFrom:  req.ValidFrom.ToTimePtr(),
		ValidTo:    req.ValidTo.ToTimePtr(),
		VpnIP:      req.VpnIP,
		CreatedBy:  createdBy,
	}

	if err := database.GetDB().Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

// GetByID gets a user by ID
func (s *UserService) GetByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := database.GetDB().Preload("Manager").First(&user, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetByUsername gets a user by username
func (s *UserService) GetByUsername(username string) (*models.User, error) {
	var user models.User
	if err := database.GetDB().Preload("Manager").First(&user, "username = ?", username).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// Update updates a user
func (s *UserService) Update(id uuid.UUID, req *dto.UpdateUserRequest, updatedBy uuid.UUID) (*models.User, error) {
	user, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})

	if req.FirstName != "" {
		updates["first_name"] = req.FirstName
	}
	if req.MiddleName != "" {
		updates["middle_name"] = req.MiddleName
	}
	if req.LastName != "" {
		updates["last_name"] = req.LastName
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Telephone != "" {
		updates["telephone"] = req.Telephone
	}
	if req.Role != "" {
		updates["role"] = req.Role
	}
	if req.ManagerID != nil {
		updates["manager_id"] = req.ManagerID
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.ValidFrom != nil {
		updates["valid_from"] = req.ValidFrom.ToTimePtr()
	}
	if req.ValidTo != nil {
		updates["valid_to"] = req.ValidTo.ToTimePtr()
	}
	if req.VpnIP != nil {
		updates["vpn_ip"] = *req.VpnIP
	}

	updates["updated_by"] = updatedBy

	if err := database.GetDB().Model(user).Updates(updates).Error; err != nil {
		return nil, err
	}

	return s.GetByID(id)
}

// UpdateProfile updates a user's own profile (limited fields)
func (s *UserService) UpdateProfile(id uuid.UUID, req *dto.UpdateProfileRequest, updatedBy uuid.UUID) (*models.User, error) {
	user, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})

	if req.FirstName != "" {
		updates["first_name"] = req.FirstName
	}
	if req.MiddleName != "" {
		updates["middle_name"] = req.MiddleName
	}
	if req.LastName != "" {
		updates["last_name"] = req.LastName
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Telephone != "" {
		updates["telephone"] = req.Telephone
	}

	updates["updated_by"] = updatedBy

	if err := database.GetDB().Model(user).Updates(updates).Error; err != nil {
		return nil, err
	}

	return s.GetByID(id)
}

// UpdatePassword updates a user's password
func (s *UserService) UpdatePassword(id uuid.UUID, currentPassword, newPassword string, updatedBy uuid.UUID) error {
	user, err := s.GetByID(id)
	if err != nil {
		return err
	}

	if !VerifyPassword(currentPassword, user.Password) {
		return ErrInvalidCredentials
	}

	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	return database.GetDB().Model(user).Updates(map[string]interface{}{
		"password":   hashedPassword,
		"updated_by": updatedBy,
	}).Error
}

// ResetPassword replaces a user's password without requiring their current password.
// Authorization is enforced by the handler route and is restricted to administrators.
func (s *UserService) ResetPassword(id uuid.UUID, newPassword string, updatedBy uuid.UUID) error {
	user, err := s.GetByID(id)
	if err != nil {
		return err
	}

	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	return database.GetDB().Model(user).Updates(map[string]interface{}{
		"password":   hashedPassword,
		"updated_by": updatedBy,
	}).Error
}

// Delete soft deletes a user
func (s *UserService) Delete(id uuid.UUID) error {
	return database.GetDB().Delete(&models.User{}, "id = ?", id).Error
}

// List lists users with pagination
func (s *UserService) List(page, pageSize int, role models.Role, userID *uuid.UUID) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := database.GetDB().Model(&models.User{}).Preload("Manager")

	// Filter based on role
	switch role {
	case models.RoleAdmin:
		// Admins can see all users
	case models.RoleManager:
		// Managers can only see users they manage (their subordinates)
		if userID != nil {
			query = query.Where("manager_id = ?", userID)
		}
	case models.RoleUser:
		// Users can only see themselves
		if userID != nil {
			query = query.Where("id = ?", userID)
		}
	}

	// Count total
	query.Count(&total)

	// Paginate
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetManagedUsers gets users managed by a specific manager
func (s *UserService) GetManagedUsers(managerID uuid.UUID, page, pageSize int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := database.GetDB().Model(&models.User{}).Where("manager_id = ?", managerID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
