package handler

import (
	"net/http"
	"solution/User"
	"solution/service"
	"solution/utils"

	"github.com/gin-gonic/gin"
)

type Authuctions struct {
	service   *service.UserService
	jwtSecret string
}

func NewAuthHandler(userService *service.UserService, jwtSecret string) *Authuctions {
	return &Authuctions{
		service:   userService,
		jwtSecret: jwtSecret,
	}
}

// POST /api/v1/auth/register

func (r *Authuctions) Register(c *gin.Context) {
	var getq User.CreateUserRequest

	if err := c.ShouldBindJSON(&getq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_JSON",
			"message": "Invalid JSON: " + err.Error(),
			"path":    "/api/v1/register",
		})
		return
	}

	if getq.ROLE == "" {
		getq.ROLE = "USER"
	}

	exists, err := r.service.CheckEmailExist(c.Request.Context(), getq.GMAIL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "INTERNAL_ERROR",
			"message": "Failed to check email",
		})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"code":    "EMAIL_EXISTS",
			"message": "Email already exists",
		})
		return
	}

	user, err := r.service.NewUser(c.Request.Context(), &getq)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"code":    "CREATE_ERROR",
			"message": err.Error(),
		})
		return
	}

	token, expiresIn, err := utils.GenerateJWT(user.ID, user.ROLE, r.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "TOKEN_GENERATION_ERROR",
			"message": "Failed to generate token",
		})
		return
	}

	users := User.TokenUser{
		AccessToken: token,
		ExpiresIn:   expiresIn,
		User:        user,
	}

	c.JSON(http.StatusCreated, users)
}

func (r *Authuctions) Login(c *gin.Context) {
	var h User.LoginUser

	if err := c.ShouldBindJSON(&h); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	user, err := r.service.Authenticate(c.Request.Context(), h.GMAIL, h.PASSWORD)
	if err != nil {
		switch err.Error() {
		case "Invalid Gmail", "Invalid password", "user not found", "invalid password":
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    "UNAUTHORIZED",
				"message": "Invalid email or password",
			})
			return
		case "user deactivated":
			c.JSON(http.StatusLocked, gin.H{
				"code":    "USER_INACTIVE",
				"message": "User account is deactivated",
			})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    "INTERNAL_SERVER_ERROR",
				"message": "Authentication failed",
			})
			return
		}
	}

	token, expiresIn, err := utils.GenerateJWT(user.ID, user.ROLE, r.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "TOKEN_GENERATION_ERROR",
			"message": "Failed to generate token",
		})
		return
	}

	users := User.TokenUser{
		AccessToken: token,
		ExpiresIn:   expiresIn,
		User:        user,
	}

	c.JSON(http.StatusOK, users)
}
