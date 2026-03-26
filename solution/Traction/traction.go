package Traction

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleUser  UserRole = "USER"
	RoleAdmin UserRole = "ADMIN"
)

type Location struct {
	ID        int64     `json:"id,omitempty" db:"id"`
	Country   string    `json:"country,omitempty" db:"country"`
	City      string    `json:"city,omitempty" db:"city"`
	Latitude  *float64  `json:"latitude,omitempty" db:"latitude"`
	Longitude *float64  `json:"longitude,omitempty" db:"longitude"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}

type TransactionStatus string

const (
	StatusApproved TransactionStatus = "APPROVED"
	StatusDeclined TransactionStatus = "DECLINED"
)

type Transaction struct {
	ID                   uuid.UUID `json:"id"`
	UserID               uuid.UUID `json:"userId"`
	Amount               float64   `json:"amount"`
	Currency             string    `json:"currency"`
	Status               string    `json:"status"`
	MerchantID           *string   `json:"merchantId,omitempty"`
	MerchantCategoryCode *string   `json:"merchantCategoryCode,omitempty"`
	Timestamp            time.Time `json:"timestamp"`
	IPAddress            *string   `json:"ipAddress,omitempty"`
	DeviceID             *string   `json:"deviceId,omitempty"`
	Channel              *string   `json:"channel,omitempty" db:"channel"`
	Location             *Location `json:"location,omitempty"`
	Metadata             []byte    `json:"-"`
	IsFraud              bool      `json:"isFraud"`
	CreatedAt            time.Time `json:"createdAt"`
}

type Rule struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	DSL         string    `json:"dsl" db:"dsl"`
	Priority    int       `json:"priority" db:"priority"`
	Enabled     bool      `json:"enabled" db:"enabled"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time `json:"updatedAt" db:"updated_at"`
}

type RuleHistory struct {
	ID            uuid.UUID `json:"id" db:"id"`
	TransactionID uuid.UUID `json:"transactionId" db:"transaction_id"`
	RuleID        uuid.UUID `json:"ruleId" db:"rule_id"`
	Matched       bool      `json:"matched" db:"matched"`
	Description   string    `json:"description" db:"description"`
	CreatedAt     time.Time `json:"createdAt" db:"created_at"`
}

type CreateTransactionRequest struct {
	UserID               *uuid.UUID             `json:"userId,omitempty"`
	Amount               float64                `json:"amount"`
	Currency             string                 `json:"currency"`
	Timestamp            time.Time              `json:"timestamp" binding:"required"`
	MerchantID           *string                `json:"merchantId,omitempty"`
	MerchantCategoryCode *string                `json:"merchantCategoryCode,omitempty"`
	IPAddress            *string                `json:"ipAddress,omitempty"`
	DeviceID             *string                `json:"deviceId,omitempty"`
	Channel              *string                `json:"channel,omitempty"`
	Location             *LocationRequest       `json:"location,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
}

type LocationRequest struct {
	Country   string   `json:"country,omitempty"`
	City      string   `json:"city,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

type CreateTransactionResponse struct {
	Transaction *TransactionResponse `json:"transaction"`
	RuleResults []RuleResult         `json:"ruleResults"`
}

type TransactionResponse struct {
	ID                   uuid.UUID              `json:"id"`
	UserID               uuid.UUID              `json:"userId"`
	Amount               float64                `json:"amount"`
	Currency             string                 `json:"currency"`
	Status               string                 `json:"status"`
	MerchantID           *string                `json:"merchantId,omitempty"`
	MerchantCategoryCode *string                `json:"merchantCategoryCode,omitempty"`
	Timestamp            time.Time              `json:"timestamp"`
	IPAddress            *string                `json:"ipAddress,omitempty"`
	DeviceID             *string                `json:"deviceId,omitempty"`
	Channel              *string                `json:"channel,omitempty"`
	Location             *LocationResponse      `json:"location,omitempty"`
	IsFraud              bool                   `json:"isFraud"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt            time.Time              `json:"createdAt"`
}

type LocationResponse struct {
	Country   string   `json:"country,omitempty"`
	City      string   `json:"city,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

type RuleResult struct {
	RuleID      uuid.UUID `json:"ruleId"`
	RuleName    string    `json:"ruleName"`
	Priority    int       `json:"priority"`
	Enabled     bool      `json:"enabled"`
	Matched     bool      `json:"matched"`
	Description string    `json:"description"`
}

type BatchCreateTransactionRequest struct {
	Items []CreateTransactionRequest `json:"items"`
}

type BatchCreateTransactionResponse struct {
	Items []BatchItemResult `json:"items"`
}

type BatchItemResult struct {
	Index    int                `json:"index"`
	Decision *TransactionResult `json:"decision,omitempty"`
	Error    *BatchError        `json:"error,omitempty"`
}

type TransactionResult struct {
	Transaction *TransactionResponse `json:"transaction"`
	RuleResults []RuleResult         `json:"ruleResults"`
}

type BatchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
