package handler

import (
	"context"
	"fmt"
	"net/http"
	"solution/User"
	"solution/service"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type HandlerUser struct {
	service  *service.UserService
	jwtToken string
}

func NewHandler(service *service.UserService, jwtTokenm string) *HandlerUser {
	return &HandlerUser{
		service:  service,
		jwtToken: jwtTokenm,
	}
}

// Middleware для проверки JWT
func (r *HandlerUser) ProvToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Missing Authorization header",
			})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "INVALID_TOKEN_FORMAT",
				"message": "Authorization header must start with 'Bearer '",
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(r.jwtToken), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		userID, ok := claims["sub"].(string)
		if !ok {

			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
			c.Abort()
			return
		}

		userRole, ok := claims["role"].(string)
		if !ok {
			fmt.Printf("WARNING: 'role' claim missing. Claims: %v\n", claims)
			userRole = "USER"
		}
		fmt.Printf("UserRole from token: %s\n", userRole)

		user, err := r.service.GetUserByID(c.Request.Context(), userID)
		if err != nil || !user.IsActive {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not active or not found"})
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Set("userRole", userRole)
		c.Set("user", user)

		c.Next()
	}
}

// медот для админских прав

func (r *HandlerUser) AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("userRole")
		if !exists || role != "ADMIN" {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    "FORBIDDEN",
				"message": "Admin access required",
				"path":    c.Request.URL.Path,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// /api/v1/users/me
func (r *HandlerUser) UsersMe(c *gin.Context) {

	userValue, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "User not found in context",
			"path":    "/api/v1/users/me",
		})
		return
	}

	c.JSON(http.StatusOK, userValue)
}

// PUT /api/v1/users/Update
func (r *HandlerUser) UpdateUSerID(c *gin.Context) {

	currentUserIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "User ID not found"})
		return
	}

	currentUserRoleValue, exists := c.Get("userRole")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "User role not found"})
		return
	}

	currentUserID := currentUserIDValue.(string)
	currentUserRole := currentUserRoleValue.(string)

	targetID := c.Param("id")
	if targetID == "" || targetID == "me" {
		targetID = currentUserID
	}

	var updateReq User.UpdateUserRequest
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}

	if err := r.validateUpdateRequest(updateReq, currentUserRole); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	ctx = context.WithValue(ctx, "userID", currentUserID)
	ctx = context.WithValue(ctx, "userRole", currentUserRole)

	updatedUser, err := r.service.UpdateUser(ctx, targetID, &updateReq)
	if err != nil {
		fmt.Printf("ERROR UpdateUser service: %v\n", err)

		if strings.Contains(err.Error(), "unauthorized") {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": err.Error()})
		} else if strings.Contains(err.Error(), "forbidden") {
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": err.Error()})
		} else if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"code": "USER_NOT_FOUND", "message": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "UPDATE_ERROR", "message": err.Error()})
		}
		return
	}

	fmt.Println("SUCCESS: Update completed")
	c.JSON(http.StatusOK, updatedUser)
}

// /api/v1/users/{id}
func (r *HandlerUser) UserID(c *gin.Context) {
	currentUserIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	currentUserID, _ := currentUserIDValue.(string)

	currentUserRoleValue, exists := c.Get("userRole")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User role not found in context"})
		return
	}
	currentUserRole, ok := currentUserRoleValue.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user role type"})
		return
	}

	targetID := c.Param("id")

	if currentUserRole == "USER" && currentUserID != targetID {
		actualPath := "/api/v1/users/" + targetID
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "USER can only access own profile",
			"path":    actualPath,
		})
		return
	}

	user, err := r.service.GetUserByID(c.Request.Context(), targetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "USER_NOT_FOUND",
			"message": "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, user)
}

// api/v1/users

func (r *HandlerUser) AllUsers(c *gin.Context) {
	const path = "/api/v1/users"
	pageStr := c.DefaultQuery("page", "0")
	sizeStr := c.DefaultQuery("size", "20")
	userRoleValue, exists := c.Get("userRole")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated", "path": "/api/v1/users"})
		return
	}

	userRole, ok := userRoleValue.(string)
	if !ok || userRole != "ADMIN" {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "Not authenticated",
			"path":    "/api/v1/users",
		})
		return
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_PAGE",
			"message": "Admin access requiredr",
			"path":    path,
		})
		return
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 1 || size > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_SIZE",
			"message": "Page must be a non-negative integer",
			"path":    path,
		})
		return
	}

	users, total, err := r.service.ListUsers(c.Request.Context(), page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "path": path})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": users,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

func (r *HandlerUser) CreateUser(c *gin.Context) {
	userRoleValue, exists := c.Get("userRole")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated", "path": "/api/v1/users"})
		return
	}

	userRole, ok := userRoleValue.(string)
	if !ok || userRole != "ADMIN" {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "Only ADMIN can create users",
			"path":    "/api/v1/users",
		})
		return
	}

	var creatuser User.CreateUserRequest

	if err := c.ShouldBindJSON(&creatuser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": err.Error(),
			"path":    "/api/v1/users",
		})
		return
	}

	if creatuser.ROLE == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"code":    "MISSING_ROLE",
			"message": "Field 'role' is required",
			"path":    "/api/v1/users",
		})
		return
	}

	if creatuser.ROLE != "USER" && creatuser.ROLE != "ADMIN" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"code":    "INVALID_ROLE",
			"message": "Role must be either USER or ADMIN",
			"path":    "/api/v1/users",
		})
		return
	}

	exists, err := r.service.CheckEmailExist(c.Request.Context(), creatuser.GMAIL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "path": "/api/v1/users"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"code":    "EMAIL_EXISTS",
			"message": "Email already exists",
			"path":    "/api/v1/users",
		})
		return
	}

	user, err := r.service.NewUser(c.Request.Context(), &creatuser)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"code":    "CREATE_ERROR",
			"message": err.Error(),
			"path":    "/api/v1/users",
		})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// /api/v1/users/{id}
func (h *HandlerUser) DeactivateUser(c *gin.Context) {
	targetID := c.Param("id")
	userRoleValue, exists := c.Get("userRole")
	if !exists {
		actualPath := "/api/v1/users/" + targetID
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated", "path": actualPath})
		return
	}

	currentUserRole, _ := userRoleValue.(string)

	currentUserIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}
	currentUserID, _ := currentUserIDValue.(string)

	if currentUserRole == "USER" && currentUserID != targetID {
		actualPath := "/api/v1/users/" + targetID
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "USER can only access own profile",
			"path":    actualPath,
		})
		return
	}

	err := h.service.DeactivateUser(c.Request.Context(), targetID)
	if err != nil {
		if err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    "USER_NOT_FOUND",
				"message": "User not found",
				"path":    fmt.Sprintf("/api/v1/users/%s", targetID),
			})
			return
		}

		if err.Error() == "user is already deactivated" {
			c.Status(http.StatusNoContent)
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (r *HandlerUser) validateUpdateRequest(req User.UpdateUserRequest, userRole string) error {
	if req.FULLNAME == "" {
		return fmt.Errorf("fullName cannot be empty")
	}
	if len(req.FULLNAME) < 2 || len(req.FULLNAME) > 200 {
		return fmt.Errorf("fullName must be between 2 and 200 characters")
	}

	if req.AGE != nil {
		if *req.AGE < 18 || *req.AGE > 120 {
			return fmt.Errorf("age must be between 18 and 120")
		}
	}

	if req.REGION != nil && len(*req.REGION) > 32 {
		return fmt.Errorf("region must be at most 32 characters")
	}

	if req.GENDER != nil {
		validGenders := map[string]bool{
			"MALE":   true,
			"FEMALE": true,
		}
		if !validGenders[*req.GENDER] {
			return fmt.Errorf("gender must be either MALE or FEMALE")
		}
	}

	if req.MaritalStatus != nil {
		validStatuses := map[string]bool{
			"SINGLE":   true,
			"MARRIED":  true,
			"DIVORCED": true,
			"WIDOWED":  true,
		}
		if !validStatuses[*req.MaritalStatus] {
			return fmt.Errorf("maritalStatus must be one of: SINGLE, MARRIED, DIVORCED, WIDOWED")
		}
	}

	if userRole == "ADMIN" && req.ROLE != nil {
		if *req.ROLE != "USER" && *req.ROLE != "ADMIN" {
			return fmt.Errorf("role must be either USER or ADMIN")
		}
	}

	return nil
}
