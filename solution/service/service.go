package service

import (
	"context"
	"fmt"
	"log"
	"solution/User"
	"solution/repository"
	"solution/utils"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo      repository.UserRepository
	jwtSecret     string
	adminEmail    string
	adminPassword string
	adminUsername string
}

func NewUserService(
	userRepo repository.UserRepository,
	jwtSecret, adminPassword, adminEmail, adminUsername string,
) *UserService {
	return &UserService{
		userRepo:      userRepo,
		jwtSecret:     jwtSecret,
		adminEmail:    adminEmail,
		adminPassword: adminPassword,
		adminUsername: adminUsername,
	}
}

func (s *UserService) CreateInitialAdmin(ctx context.Context) error {
	existingAdmin, err := s.userRepo.GetByEmail(ctx, s.adminEmail)
	if err != nil {

		if !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("ошибка при проверке администратора: %w", err)
		}

	}

	if existingAdmin != nil {
		log.Printf("Администратор %s уже существует (ID: %s)", s.adminEmail, existingAdmin.ID)
		return nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(s.adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("ошибка хэширования пароля: %w", err)
	}

	age := 35
	region := "RU-MOW"

	admin := &User.User{
		ID:            uuid.New().String(),
		GMAIL:         s.adminEmail,
		PASSWORD:      string(hashedPassword),
		FULLNAME:      s.adminUsername,
		AGE:           age,
		REGION:        region,
		GENDER:        "MALE",
		ROLE:          "ADMIN",
		IsActive:      true,
		MaritalStatus: "SINGLE",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	log.Printf("Создаем администратора: %s (%s)", admin.FULLNAME, admin.GMAIL)

	err = s.userRepo.Create(ctx, admin)
	if err != nil {
		return fmt.Errorf("ошибка создания администратора в БД: %w", err)
	}

	log.Printf("Администратор успешно создан!")
	return nil
}

func (s *UserService) NewUser(ctx context.Context, r *User.CreateUserRequest) (*User.User, error) {
	exists, err := s.userRepo.CheckEmailExist(ctx, r.GMAIL)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}

	if exists {
		return nil, fmt.Errorf("email already exists")
	}

	hashPassword, err := utils.HashPassword(r.PASSWORD)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &User.User{
		ID:            uuid.New().String(),
		GMAIL:         r.GMAIL,
		PASSWORD:      hashPassword,
		FULLNAME:      r.FULLNAME,
		AGE:           r.AGE,
		REGION:        r.REGION,
		GENDER:        r.GENDER,
		ROLE:          "USER",
		IsActive:      true,
		MaritalStatus: r.MaritalStatus,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user in DB: %w", err)
	}

	user.PASSWORD = ""
	return user, nil
}

func (s *UserService) CheckEmailExist(ctx context.Context, email string) (bool, error) {
	return s.userRepo.CheckEmailExist(ctx, email)
}

func (s *UserService) Authenticate(ctx context.Context, email, password string) (*User.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user deactivated")
	}

	if !utils.CheckPasswordHash(password, user.PASSWORD) {
		return nil, fmt.Errorf("invalid password")
	}

	user.PASSWORD = ""
	return user, nil
}

func (s *UserService) GetUserByID(ctx context.Context, id string) (*User.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	user.PASSWORD = ""
	return user, nil
}

func (s *UserService) ListUsers(ctx context.Context, page, size int) ([]*User.User, int, error) {
	if page < 0 {
		page = 0
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	offset := page * size

	users, err := s.userRepo.List(ctx, offset, size)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	total, err := s.userRepo.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	for _, u := range users {
		u.PASSWORD = ""
	}

	return users, total, nil
}

func (s *UserService) DeactivateUser(ctx context.Context, id string) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	user.IsActive = false
	user.UpdatedAt = time.Now()

	return s.userRepo.Update(ctx, user)
}

func (s *UserService) UpdateUser(ctx context.Context, userID string, req *User.UpdateUserRequest) (*User.User, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	currentUserIDValue := ctx.Value("userID")
	currentUserRoleValue := ctx.Value("userRole")

	if currentUserIDValue == nil || currentUserRoleValue == nil {
		return nil, fmt.Errorf("unauthorized: missing auth context")
	}

	currentUserID, ok := currentUserIDValue.(string)
	if !ok {
		return nil, fmt.Errorf("unauthorized: invalid user ID type")
	}

	currentUserRole, ok := currentUserRoleValue.(string)
	if !ok {
		return nil, fmt.Errorf("unauthorized: invalid user role type")
	}

	if currentUserRole == "USER" && currentUserID != userID {
		return nil, fmt.Errorf("forbidden: USER can only update own profile")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	user.FULLNAME = req.FULLNAME

	if req.AGE != nil {
		user.AGE = *req.AGE
	}

	if req.REGION != nil && *req.REGION != "" {
		user.REGION = *req.REGION
	}

	if req.GENDER != nil && *req.GENDER != "" {
		user.GENDER = *req.GENDER
	}

	if req.MaritalStatus != nil && *req.MaritalStatus != "" {
		user.MaritalStatus = *req.MaritalStatus
	}

	if req.ROLE != nil && *req.ROLE != "" {
		if currentUserRole != "ADMIN" {
			return nil, fmt.Errorf("forbidden: only ADMIN can update role")
		}
		user.ROLE = *req.ROLE
	}

	if req.IsActive != nil {
		if currentUserRole != "ADMIN" {
			return nil, fmt.Errorf("forbidden: only ADMIN can update active status")
		}
		user.IsActive = *req.IsActive
	}

	user.UpdatedAt = time.Now()

	if user.FULLNAME == "" {
		return nil, fmt.Errorf("validation failed: full name cannot be empty")
	}
	if user.AGE < 18 || user.AGE > 120 {
		return nil, fmt.Errorf("validation failed: age must be between 18 and 120")
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	user.PASSWORD = ""
	return user, nil
}
