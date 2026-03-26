package User

import (
	"encoding/json"
	"time"
)

type User struct {
	ID            string    `json:"id"`
	GMAIL         string    `json:"email"`
	PASSWORD      string    `json:"password,omitempty"`
	FULLNAME      string    `json:"fullName"`
	AGE           int       `json:"age"`
	REGION        string    `json:"region"`
	GENDER        string    `json:"gender"`
	ROLE          string    `json:"role"`
	IsActive      bool      `json:"isActive"`
	MaritalStatus string    `json:"maritalStatus"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
type CreateUserRequest struct {
	GMAIL         string `json:"email" binding:"required,email"`
	PASSWORD      string `json:"password" binding:"required,min=8"`
	FULLNAME      string `json:"fullName" binding:"required,min=2,max=200"`
	AGE           int    `json:"age" binding:"required,min=18,max=120"`
	REGION        string `json:"region" binding:"required,max=32"`
	GENDER        string `json:"gender" binding:"required,oneof=MALE FEMALE"`
	MaritalStatus string `json:"maritalStatus" binding:"required,oneof=SINGLE MARRIED DIVORCED WIDOWED"`
	ROLE          string `json:"role" binding:"omitempty,oneof=USER ADMIN"`
}

type LoginUser struct {
	GMAIL    string `json:"email" validate:"required,email,max=254"`
	PASSWORD string `json:"password" validate:"required,min=8,max=90"`
}

type TokenUser struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int    `json:"expiresIn"`
	User        *User  `json:"user"`
}

type UpdateUserRequest struct {
	FULLNAME      string  `json:"fullName" binding:"required"`
	AGE           *int    `json:"age"`
	REGION        *string `json:"region"`
	GENDER        *string `json:"gender"`
	MaritalStatus *string `json:"maritalStatus"`
	ROLE          *string `json:"role,omitempty"`
	IsActive      *bool   `json:"isActive,omitempty"`
}

func (t *TokenUser) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}
