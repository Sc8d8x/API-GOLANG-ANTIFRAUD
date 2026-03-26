package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"solution/Traction"
	"solution/service"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type ErrorResponse struct {
	Code        string       `json:"code"`
	Message     string       `json:"message"`
	TraceId     string       `json:"traceId"`
	Timestamp   time.Time    `json:"timestamp"`
	Path        string       `json:"path"`
	Details     interface{}  `json:"details,omitempty"`
	FieldErrors []FieldError `json:"fieldErrors,omitempty"`
}

type FieldError struct {
	Field         string      `json:"field"`
	Issue         string      `json:"issue"`
	RejectedValue interface{} `json:"rejectedValue,omitempty"`
}

type TransactionHandler struct {
	transactionService *service.TransactionServiceQQ
}

func NewTransactionHandler(transactionService *service.TransactionServiceQQ) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
	}
}

func sendError(c *gin.Context, code string, message string, fieldErrors []FieldError) {
	resp := ErrorResponse{
		Code:        code,
		Message:     message,
		TraceId:     uuid.New().String(),
		Timestamp:   time.Now().UTC(),
		Path:        c.Request.URL.Path,
		FieldErrors: fieldErrors,
	}

	status := http.StatusInternalServerError
	switch code {
	case "BAD_REQUEST":
		status = http.StatusBadRequest
	case "VALIDATION_FAILED":
		status = http.StatusUnprocessableEntity
	case "UNAUTHORIZED":
		status = http.StatusUnauthorized
	case "FORBIDDEN":
		status = http.StatusForbidden
	case "NOT_FOUND":
		status = http.StatusNotFound
	case "USER_INACTIVE":
		status = http.StatusLocked // 423
	default:
		status = http.StatusInternalServerError
	}

	c.JSON(status, resp)
}

func handleValidationErrors(c *gin.Context, err error) {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		fieldErrors := make([]FieldError, 0)
		for _, ve := range validationErrors {
			fieldName := ve.Field()
			jsonField := fieldName
			if len(fieldName) > 0 {
				jsonField = strings.ToLower(fieldName[:1]) + fieldName[1:]
			}
			fieldErrors = append(fieldErrors, FieldError{
				Field: jsonField,
				Issue: ve.Error(),
			})
		}
		sendError(c, "VALIDATION_FAILED", "Some fields failed validation", fieldErrors)
		return
	}

	sendError(c, "BAD_REQUEST", "Invalid JSON format", nil)
}

// POST /api/v1/transactions
func (h *TransactionHandler) CreateTransaction(c *gin.Context) {
	var req Traction.CreateTransactionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		handleValidationErrors(c, err)
		return
	}

	fieldErrors := validateCreateTransactionRequest(req)
	if len(fieldErrors) > 0 {
		sendError(c, "VALIDATION_FAILED", "Some fields failed validation", fieldErrors)
		return
	}

	userIDStr, exists := c.Get("userID")
	if !exists {
		sendError(c, "UNAUTHORIZED", "Missing or invalid authentication token", nil)
		return
	}

	userRoleStr, exists := c.Get("userRole")
	if !exists {
		sendError(c, "UNAUTHORIZED", "Missing or invalid authentication token", nil)
		return
	}

	userIDVal, ok := userIDStr.(string)
	if !ok {
		sendError(c, "UNAUTHORIZED", "Invalid user ID format in token", nil)
		return
	}

	userRoleVal, ok := userRoleStr.(string)
	if !ok {
		sendError(c, "UNAUTHORIZED", "Invalid user role format in token", nil)
		return
	}

	currentUserID, err := uuid.Parse(userIDVal)
	if err != nil {
		sendError(c, "BAD_REQUEST", "Invalid user ID format", nil)
		return
	}

	currentUserRole := Traction.UserRole(userRoleVal)

	response, err := h.transactionService.Create(c.Request.Context(), req, currentUserID, currentUserRole)
	if err != nil {
		errorMsg := err.Error()

		switch {
		case strings.Contains(errorMsg, "user not found"):
			sendErrorWithDetails(c, "NOT_FOUND", "User not found", gin.H{
				"userId": userIDVal,
			}, nil)
		case strings.Contains(errorMsg, "user is deactivated"):
			sendError(c, "USER_INACTIVE", "User account is inactive", nil)
		case strings.Contains(errorMsg, "userId is required for ADMIN"):
			sendError(c, "BAD_REQUEST", errorMsg, nil)
		case strings.Contains(errorMsg, "users can only create transactions for themselves"):
			sendError(c, "FORBIDDEN", "Access denied: cannot create transaction for another user", nil)
		default:
			sendError(c, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	c.JSON(http.StatusCreated, response)
}

func validateCreateTransactionRequest(req Traction.CreateTransactionRequest) []FieldError {
	var errors []FieldError

	if req.Amount < 0.01 {
		errors = append(errors, FieldError{
			Field:         "amount",
			Issue:         "Amount must be at least 0.01",
			RejectedValue: req.Amount,
		})
	} else if req.Amount > 999999999.99 {
		errors = append(errors, FieldError{
			Field:         "amount",
			Issue:         "Amount must not exceed 999999999.99",
			RejectedValue: req.Amount,
		})
	}

	if len(req.Currency) != 3 {
		errors = append(errors, FieldError{
			Field:         "currency",
			Issue:         "Currency must be exactly 3 characters",
			RejectedValue: req.Currency,
		})
	} else {
		for _, r := range req.Currency {
			if r < 'A' || r > 'Z' {
				errors = append(errors, FieldError{
					Field:         "currency",
					Issue:         "Currency must be 3 uppercase letters",
					RejectedValue: req.Currency,
				})
				break
			}
		}
	}

	now := time.Now().UTC()
	if req.Timestamp.After(now.Add(5 * time.Minute)) {
		errors = append(errors, FieldError{
			Field: "timestamp",
			Issue: "Timestamp cannot be more than 5 minutes in the future",
		})
	}

	locationErrors := validateLocationRequest(req.Location)
	errors = append(errors, locationErrors...)

	return errors
}

// GET /api/v1/transactions/{id}
func (h *TransactionHandler) GetTransaction(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		sendError(c, "BAD_REQUEST", "Invalid transaction ID format", nil)
		return
	}

	userIDStr, exists := c.Get("userID")
	if !exists {
		sendError(c, "UNAUTHORIZED", "Missing or invalid authentication token", nil)
		return
	}

	userRoleStr, exists := c.Get("userRole")
	if !exists {
		sendError(c, "UNAUTHORIZED", "Missing or invalid authentication token", nil)
		return
	}

	userIDVal, ok := userIDStr.(string)
	if !ok {
		sendError(c, "UNAUTHORIZED", "Invalid user ID format in token", nil)
		return
	}

	userRoleVal, ok := userRoleStr.(string)
	if !ok {
		sendError(c, "UNAUTHORIZED", "Invalid user role format in token", nil)
		return
	}

	userID, err := uuid.Parse(userIDVal)
	if err != nil {
		sendError(c, "BAD_REQUEST", "Invalid user ID format", nil)
		return
	}

	userRole := Traction.UserRole(userRoleVal)

	transaction, ruleResults, err := h.transactionService.GetByID(c.Request.Context(), id, userID, userRole)
	if err != nil {
		if strings.Contains(err.Error(), "transaction not found") {
			sendError(c, "NOT_FOUND", "Transaction not found", nil)
		} else if strings.Contains(err.Error(), "access denied") {
			sendError(c, "FORBIDDEN", "Access denied", nil)
		} else {
			sendError(c, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transaction": transaction,
		"ruleResults": ruleResults,
	})
}

// GET /api/v1/transactions
func (h *TransactionHandler) GetTransactions(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		sendError(c, "UNAUTHORIZED", "Missing or invalid authentication token", nil)
		return
	}

	userRoleStr, exists := c.Get("userRole")
	if !exists {
		sendError(c, "UNAUTHORIZED", "Missing or invalid authentication token", nil)
		return
	}

	userIDVal, ok := userIDStr.(string)
	if !ok {
		sendError(c, "UNAUTHORIZED", "Invalid user ID format in token", nil)
		return
	}

	userRoleVal, ok := userRoleStr.(string)
	if !ok {
		sendError(c, "UNAUTHORIZED", "Invalid user role format in token", nil)
		return
	}

	userID, err := uuid.Parse(userIDVal)
	if err != nil {
		sendError(c, "BAD_REQUEST", "Invalid user ID format", nil)
		return
	}

	userRole := Traction.UserRole(userRoleVal)

	targetUserID := userID
	if userRole == Traction.RoleAdmin {
		if userIdParam := c.Query("userId"); userIdParam != "" {
			if userIdFilter, err := uuid.Parse(userIdParam); err == nil {
				targetUserID = userIdFilter
			} else {
				sendError(c, "BAD_REQUEST", "Invalid userId query parameter", nil)
				return
			}
		}
	}

	filters := make(map[string]interface{})

	if status := c.Query("status"); status != "" {
		if status == "APPROVED" || status == "DECLINED" {
			filters["status"] = status
		} else {
			sendError(c, "BAD_REQUEST", "Invalid status value, must be APPROVED or DECLINED", nil)
			return
		}
	}

	if isFraud := c.Query("isFraud"); isFraud != "" {
		if isFraudBool, err := strconv.ParseBool(isFraud); err == nil {
			filters["isFraud"] = isFraudBool
		} else {
			sendError(c, "BAD_REQUEST", "Invalid isFraud parameter, must be true or false", nil)
			return
		}
	}

	if from := c.Query("from"); from != "" {
		if fromTime, err := time.Parse(time.RFC3339, from); err == nil {
			filters["from"] = fromTime
		} else {
			sendError(c, "BAD_REQUEST", "Invalid 'from' timestamp format, expected RFC3339", nil)
			return
		}
	}

	if to := c.Query("to"); to != "" {
		if toTime, err := time.Parse(time.RFC3339, to); err == nil {
			filters["to"] = toTime
		} else {
			sendError(c, "BAD_REQUEST", "Invalid 'to' timestamp format, expected RFC3339", nil)
			return
		}
	}

	page := 0
	size := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if pageInt, err := strconv.Atoi(pageStr); err == nil && pageInt >= 0 {
			page = pageInt
			filters["page"] = page
		} else {
			sendError(c, "BAD_REQUEST", "Invalid page parameter, must be non-negative integer", nil)
			return
		}
	}

	if sizeStr := c.Query("size"); sizeStr != "" {
		if sizeInt, err := strconv.Atoi(sizeStr); err == nil && sizeInt > 0 && sizeInt <= 100 {
			size = sizeInt
			filters["size"] = size
		} else {
			sendError(c, "BAD_REQUEST", "Invalid size parameter, must be between 1 and 100", nil)
			return
		}
	}

	transactions, total, err := h.transactionService.GetByUserID(
		c.Request.Context(),
		targetUserID,
		filters,
		userRole,
	)
	if err != nil {
		sendError(c, "INTERNAL_SERVER_ERROR", "Failed to fetch transactions", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": transactions,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

func validateLocationRequest(loc *Traction.LocationRequest) []FieldError {
	var errors []FieldError

	if loc == nil {
		return errors
	}
	if loc.Country != "" {
		if len(loc.Country) != 2 {
			errors = append(errors, FieldError{
				Field:         "location.country",
				Issue:         "Country code must be exactly 2 uppercase letters",
				RejectedValue: loc.Country,
			})
		}
	}

	hasLat := loc.Latitude != nil
	hasLon := loc.Longitude != nil

	if hasLat != hasLon {
		errors = append(errors, FieldError{
			Field: "location",
			Issue: "Latitude and longitude must be provided together",
		})
	} else if hasLat {
		lat := *loc.Latitude
		lon := *loc.Longitude

		if lat < -90 || lat > 90 {
			errors = append(errors, FieldError{
				Field:         "location.latitude",
				Issue:         "Latitude must be between -90 and 90",
				RejectedValue: lat,
			})
		}
		if lon < -180 || lon > 180 {
			errors = append(errors, FieldError{
				Field:         "location.longitude",
				Issue:         "Longitude must be between -180 and 180",
				RejectedValue: lon,
			})
		}
	}

	return errors
}

func sendErrorWithDetails(c *gin.Context, code string, message string, details interface{}, fieldErrors []FieldError) {
	resp := ErrorResponse{
		Code:        code,
		Message:     message,
		TraceId:     uuid.New().String(),
		Timestamp:   time.Now().UTC(),
		Path:        c.Request.URL.Path,
		Details:     details,
		FieldErrors: fieldErrors,
	}

	status := http.StatusInternalServerError
	switch code {
	case "BAD_REQUEST":
		status = http.StatusBadRequest
	case "VALIDATION_FAILED":
		status = http.StatusUnprocessableEntity
	case "UNAUTHORIZED":
		status = http.StatusUnauthorized
	case "FORBIDDEN":
		status = http.StatusForbidden
	case "NOT_FOUND":
		status = http.StatusNotFound
	case "USER_INACTIVE":
		status = http.StatusLocked
	default:
		status = http.StatusInternalServerError
	}

	c.JSON(status, resp)
}
