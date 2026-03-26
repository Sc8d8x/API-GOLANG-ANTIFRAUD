package handler

import (
	"bytes"
	"io/ioutil"
	"net/http"
	fraud "solution/Fraud"
	"solution/service"
	"strconv"

	"github.com/gin-gonic/gin"
	"errors"
)

type HandlerFraud struct {
	service *service.FraudService
}

func NewHandlerFraud(service *service.FraudService) *HandlerFraud {
	return &HandlerFraud{
		service: service,
	}
}

var ErrRuleAlreadyExists = errors.New("fraud rule with this name already exists")

// POST /api/v1/fraud-rules
func (h *HandlerFraud) CreateFraud(c *gin.Context) {
	const path = "/api/v1/fraud-rules"

	userRoleValue, exists := c.Get("userRole")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Not authenticated",
			"path":    path,
		})
		return
	}

	userRole, ok := userRoleValue.(string)
	if !ok || userRole != "ADMIN" {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "Only ADMIN can create fraud rules",
			"path":    path,
		})
		return
	}

	bodyData, err := c.GetRawData()
	if err != nil || len(bodyData) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": "Request body is required",
			"path":    path,
		})
		return
	}

	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyData))

	var createReq fraud.CreateFraudRequest
	if err := c.ShouldBindJSON(&createReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": "Invalid JSON: " + err.Error(),
			"path":    path,
		})
		return
	}

	if createReq.NAME == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "name is required",
			"path":    path,
		})
		return
	}

	if createReq.DLSEXPRESSION == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "dslExpression is required",
			"path":    path,
		})
		return
	}

	if createReq.PRIORITY < 1 || createReq.PRIORITY > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "priority must be between 1 and 100",
			"path":    path,
		})
		return
	}

	fraudObj, err := h.service.NewFraud(c.Request.Context(), &createReq)
	if err != nil {
		if err.Error() == "fraud with name already exists" {
			c.JSON(http.StatusConflict, gin.H{
				"code":    "RULE_NAME_ALREADY_EXISTS",
				"message": "Rule with this name already exists",
				"path":    path,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "INTERNAL_SERVER_ERROR",
			"message": "Failed to create fraud rule",
			"path":    path,
		})
		return
	}

	c.JSON(http.StatusCreated, fraudObj)
}

// GET /api/v1/fraud-rules
func (h *HandlerFraud) AllFraud(c *gin.Context) {

	userRoleValue, exists := c.Get("userRole")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Not authenticated",
		})
		return
	}

	userRole, ok := userRoleValue.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Invalid user role",
		})
		return
	}

	if userRole != "ADMIN" {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "Only ADMIN can view fraud rules",
		})
		return
	}

	pageStr := c.DefaultQuery("page", "0")
	sizeStr := c.DefaultQuery("size", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_PAGE",
			"message": "Page must be a non-negative integer",
		})
		return
	}

	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 1 || size > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_SIZE",
			"message": "Size must be an integer between 1 and 100",
		})
		return
	}

	frauds, total, err := h.service.ListFraud(c.Request.Context(), page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": frauds,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GET /api/v1/fraud-rules/{id}
func (h *HandlerFraud) GetBYID(c *gin.Context) {

	userRoleValue, exists := c.Get("userRole")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Not authenticated",
		})
		return
	}

	userRole, ok := userRoleValue.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Invalid user role",
		})
		return
	}

	if userRole != "ADMIN" {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "Only ADMIN can view fraud rules",
		})
		return
	}

	targetID := c.Param("id")
	if targetID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_ID",
			"message": "Fraud rule ID is required",
		})
		return
	}

	fraud, err := h.service.GetBYID(c.Request.Context(), targetID)
	if err != nil {
		if err.Error() == "fraud not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    "NOT_FOUND",
				"message": "Fraud rule not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, fraud)
}

// PUT /api/v1/fraud-rules/{id}
func (h *HandlerFraud) UpdateFraudID(c *gin.Context) {

	userRoleValue, exists := c.Get("userRole")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Not authenticated",
		})
		return
	}

	userRole, ok := userRoleValue.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Invalid user role",
		})
		return
	}

	if userRole != "ADMIN" {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "Only ADMIN can update fraud rules",
		})
		return
	}

	targetID := c.Param("id")
	if targetID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_ID",
			"message": "Fraud rule ID is required",
		})
		return
	}

	var updateReq fraud.UpdateFraud
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": err.Error(),
		})
		return
	}

	updatedFraud, err := h.service.Update(c.Request.Context(), targetID, &updateReq)
	if err != nil {
		if err.Error() == "fraud not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    "NOT_FOUND",
				"message": "Fraud rule not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, updatedFraud)
}

// DELETE /api/v1/fraud-rules/{id}
func (h *HandlerFraud) DeactivateFraudID(c *gin.Context) {

	userRoleValue, exists := c.Get("userRole")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Not authenticated",
		})
		return
	}

	userRole, ok := userRoleValue.(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Invalid user role",
		})
		return
	}

	if userRole != "ADMIN" {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "Only ADMIN can deactivate fraud rules",
		})
		return
	}

	targetID := c.Param("id")
	if targetID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_ID",
			"message": "Fraud rule ID is required",
		})
		return
	}

	err := h.service.DeactivateFraud(c.Request.Context(), targetID)
	if err != nil {
		if err.Error() == "fraud not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    "NOT_FOUND",
				"message": "Fraud rule not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// POST /api/v1/fraud-rules/validate
func (h *HandlerFraud) ValiteDLS(c *gin.Context) {
	userRoleValue, exists := c.Get("userRole")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Not authenticated",
		})
		return
	}

	userRole, ok := userRoleValue.(string)
	if !ok || userRole != "ADMIN" {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "Only ADMIN can validate DSL expressions",
		})
		return
	}

	var req fraud.ValidateDSLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": err.Error(),
		})
		return
	}

	response, err := h.service.ValiteDLS(c.Request.Context(), req.DSLExpression)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
