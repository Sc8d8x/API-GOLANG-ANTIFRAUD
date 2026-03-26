package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"solution/Traction"
	"solution/repository"
	"time"

	"github.com/google/uuid"
)

type TransactionServiceQQ struct {
	transactionRepo repository.TransactionRepository
	userRepo        repository.UserRepository
	ruleRepo        repository.RuleRepository
}

func NewTransactionServiceQQ(
	transactionRepo repository.TransactionRepository,
	userRepo repository.UserRepository,
	ruleRepo repository.RuleRepository,
) *TransactionServiceQQ {
	return &TransactionServiceQQ{
		transactionRepo: transactionRepo,
		userRepo:        userRepo,
		ruleRepo:        ruleRepo,
	}
}

func (s *TransactionServiceQQ) Create(
	ctx context.Context,
	req Traction.CreateTransactionRequest,
	currentUserID uuid.UUID,
	currentUserRole Traction.UserRole,
) (*Traction.CreateTransactionResponse, error) {

	var targetUserID uuid.UUID
	if currentUserRole == Traction.RoleAdmin {
		if req.UserID == nil {
			return nil, fmt.Errorf("userId is required for ADMIN")
		}
		targetUserID = *req.UserID
	} else {
		if req.UserID != nil {
			if *req.UserID != currentUserID {
				return nil, fmt.Errorf("users can only create transactions for themselves")
			}
		}
		targetUserID = currentUserID
	}

	user, err := s.userRepo.GetByID(ctx, targetUserID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user is deactivated")
	}

	metadataBytes := []byte("{}")
	if req.Metadata != nil {
		var err error
		metadataBytes, err = json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	now := time.Now().UTC()
	transaction := &Traction.Transaction{
		ID:                   uuid.New(),
		UserID:               targetUserID,
		Amount:               req.Amount,
		Currency:             req.Currency,
		Timestamp:            req.Timestamp,
		MerchantID:           req.MerchantID,
		MerchantCategoryCode: req.MerchantCategoryCode,
		IPAddress:            req.IPAddress,
		DeviceID:             req.DeviceID,
		Channel:              req.Channel,
		CreatedAt:            now,
		Metadata:             metadataBytes,
	}

	if req.Location != nil {
		transaction.Location = &Traction.Location{
			Country:   req.Location.Country,
			City:      req.Location.City,
			Latitude:  req.Location.Latitude,
			Longitude: req.Location.Longitude,
		}
	}

	rules, err := s.ruleRepo.GetAllActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rules: %w", err)
	}

	var ruleResults []Traction.RuleResult
	isFraud := false
	if isFraud {
		transaction.Status = "DECLINED"
		transaction.IsFraud = true
	} else {
		transaction.Status = "APPROVED"
		transaction.IsFraud = false
	}

	for _, rule := range rules {
		ruleResults = append(ruleResults, Traction.RuleResult{
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			Priority:    rule.Priority,
			Enabled:     rule.Enabled,
			Matched:     false,
			Description: fmt.Sprintf("Правило '%s': пропущено (Tier 0)", rule.Name),
		})
	}

	transaction.Status = string(Traction.StatusApproved)

	if err := s.transactionRepo.CreateWithResults(ctx, transaction, ruleResults); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	return &Traction.CreateTransactionResponse{
		Transaction: s.convertToResponse(transaction),
		RuleResults: ruleResults,
	}, nil
}

func (s *TransactionServiceQQ) GetByID(
	ctx context.Context,
	transactionID uuid.UUID,
	currentUserID uuid.UUID,
	currentUserRole Traction.UserRole,
) (*Traction.TransactionResponse, []Traction.RuleResult, error) {

	tx, ruleResults, err := s.transactionRepo.GetByIDWithResults(ctx, transactionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("transaction not found")
		}
		return nil, nil, err
	}

	if currentUserRole == Traction.RoleUser && tx.UserID != currentUserID {
		return nil, nil, fmt.Errorf("access denied")
	}

	return s.convertToResponse(tx), ruleResults, nil
}

func (s *TransactionServiceQQ) GetByUserID(
	ctx context.Context,
	userID uuid.UUID,
	filters map[string]interface{},
	currentUserRole Traction.UserRole,
) ([]Traction.TransactionResponse, int64, error) {

	transactions, total, err := s.transactionRepo.GetByUserID(ctx, userID, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get transactions: %w", err)
	}

	responses := make([]Traction.TransactionResponse, len(transactions))
	for i, tx := range transactions {
		responses[i] = *s.convertToResponse(&tx)
	}
	return responses, total, nil
}

func (s *TransactionServiceQQ) convertToResponse(tx *Traction.Transaction) *Traction.TransactionResponse {
	resp := &Traction.TransactionResponse{
		ID:                   tx.ID,
		UserID:               tx.UserID,
		Amount:               tx.Amount,
		Currency:             tx.Currency,
		Status:               tx.Status,
		MerchantID:           tx.MerchantID,
		MerchantCategoryCode: tx.MerchantCategoryCode,
		Timestamp:            tx.Timestamp,
		IPAddress:            tx.IPAddress,
		DeviceID:             tx.DeviceID,
		Channel:              tx.Channel,
		IsFraud:              tx.IsFraud,
		CreatedAt:            tx.CreatedAt,
	}

	if tx.Location != nil {
		resp.Location = &Traction.LocationResponse{
			Country:   tx.Location.Country,
			City:      tx.Location.City,
			Latitude:  tx.Location.Latitude,
			Longitude: tx.Location.Longitude,
		}
	}

	if len(tx.Metadata) > 0 && string(tx.Metadata) != "{}" {
		var meta map[string]interface{}
		if err := json.Unmarshal(tx.Metadata, &meta); err == nil {
			resp.Metadata = meta
		}
	}

	return resp
}

func (s *TransactionServiceQQ) CreateBatch(
	ctx context.Context,
	req *Traction.CreateTransactionRequest,
	currentUserID uuid.UUID,
	currentUserRole Traction.UserRole,
) (*Traction.TransactionResponse, []Traction.RuleResult, error) {

	var targetUserID uuid.UUID
	if currentUserRole == Traction.RoleAdmin {
		if req.UserID == nil {
			return nil, nil, fmt.Errorf("userId is required for ADMIN")
		}
		targetUserID = *req.UserID
	} else {

		targetUserID = currentUserID
	}

	user, err := s.userRepo.GetByID(ctx, targetUserID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("user not found")
		}
		return nil, nil, fmt.Errorf("database error: %w", err)
	}
	if !user.IsActive {
		return nil, nil, fmt.Errorf("user is deactivated")
	}

	metadataBytes := []byte("{}")
	if req.Metadata != nil {
		metadataBytes, err = json.Marshal(req.Metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	now := time.Now().UTC()
	transaction := &Traction.Transaction{
		ID:                   uuid.New(),
		UserID:               targetUserID,
		Amount:               req.Amount,
		Currency:             req.Currency,
		Status:               string(Traction.StatusApproved),
		Timestamp:            req.Timestamp,
		MerchantID:           req.MerchantID,
		MerchantCategoryCode: req.MerchantCategoryCode,
		IPAddress:            req.IPAddress,
		DeviceID:             req.DeviceID,
		Channel:              req.Channel,
		CreatedAt:            now,
		Metadata:             metadataBytes,
		IsFraud:              false,
	}

	if req.Location != nil {
		transaction.Location = &Traction.Location{
			Country:   req.Location.Country,
			City:      req.Location.City,
			Latitude:  req.Location.Latitude,
			Longitude: req.Location.Longitude,
		}
	}

	rules, err := s.ruleRepo.GetAllActive(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get active rules: %w", err)
	}

	var ruleResults []Traction.RuleResult
	for _, rule := range rules {
		ruleResults = append(ruleResults, Traction.RuleResult{
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			Priority:    rule.Priority,
			Enabled:     rule.Enabled,
			Matched:     false,
			Description: fmt.Sprintf("Правило '%s': пропущено (Tier 0)", rule.Name),
		})
	}

	if err := s.transactionRepo.CreateWithResults(ctx, transaction, ruleResults); err != nil {
		return nil, nil, fmt.Errorf("failed to save transaction: %w", err)
	}

	response := s.convertToResponse(transaction)

	return response, ruleResults, nil
}
