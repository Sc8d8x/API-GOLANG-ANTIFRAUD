package fraud

import (
	"time"
)

type Fraud struct {
	ID            string    `json:"id"`
	NAME          string    `json:"name"`
	DESCRIPTION   string    `json:"description,omitempty"`
	DLSEXPRESSION string    `json:"dslExpression"`
	ENABLED       bool      `json:"enabled"`
	PRIORITY      int       `json:"priority"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type CreateFraudRequest struct {
	NAME          string `json:"name" binding:"required"`
	DESCRIPTION   string `json:"description" binding:"required"`
	DLSEXPRESSION string `json:"dslExpression" binding:"required"`
	ENABLED       bool   `json:"enabled" binding:"required"`
	PRIORITY      int    `json:"priority" binding:"required,min=1,max=100"`
}

type UpdateFraud struct {
	NAME          *string `json:"name"`
	DESCRIPTION   *string `json:"description"`
	DLSEXPRESSION *string `json:"dslExpression"`
	ENABLED       *bool   `json:"enabled"`
	PRIORITY      *int    `json:"priority,omitempty"`
}
