package service

import (
	"context"
	"fmt"
	fraud "solution/Fraud"
	"solution/repository"
	"time"

	"errors"

	"github.com/google/uuid"
)

type FraudService struct {
	fraudRepo repository.FraudRepository
	validator *DSLValidator
}

func NewFraudService(fraudRepo repository.FraudRepository) *FraudService {
	return &FraudService{
		fraudRepo: fraudRepo,
		validator: NewDSLValidator(),
	}
}

var ErrRuleAlreadyExists = errors.New("fraud rule already exists")

func (s *FraudService) NewFraud(ctx context.Context, r *fraud.CreateFraudRequest) (*fraud.Fraud, error) {
	exist, err := s.fraudRepo.CheckNameExist(ctx, r.NAME)
	if err != nil {
		return nil, fmt.Errorf("failed create new fraud: %w", err)
	}

	if exist {
		return nil, ErrRuleAlreadyExists
	}

	fraud := &fraud.Fraud{
		ID:            uuid.New().String(),
		NAME:          r.NAME,
		DESCRIPTION:   r.DESCRIPTION,
		DLSEXPRESSION: r.DLSEXPRESSION,
		ENABLED:       r.ENABLED,
		PRIORITY:      r.PRIORITY,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err = s.fraudRepo.Create(ctx, fraud)

	if err != nil {
		return nil, fmt.Errorf("failed to save fraud to database: %w", err)
	}

	return fraud, nil

}

func (s *FraudService) GetBYID(ctx context.Context, id string) (*fraud.Fraud, error) {
	fraud, err := s.fraudRepo.GetBYID(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("get fraud by id: %w", err)
	}

	return fraud, nil
}

func (s *FraudService) DeactivateFraud(ctx context.Context, id string) error {
	fraud, err := s.fraudRepo.GetBYID(ctx, id)

	if err != nil {
		return fmt.Errorf("get fraud: %w", err)
	}

	fraud.ENABLED = false
	fraud.UpdatedAt = time.Now()

	return s.fraudRepo.Update(ctx, fraud)
}

func (s *FraudService) Update(ctx context.Context, id string, r *fraud.UpdateFraud) (*fraud.Fraud, error) {
	fraud, err := s.fraudRepo.GetBYID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fraud not found: %w", err)
	}

	if r.NAME != nil && *r.NAME != "" {
		fraud.NAME = *r.NAME
	}

	if r.DESCRIPTION != nil && *r.DESCRIPTION != "" {
		fraud.DESCRIPTION = *r.DESCRIPTION
	}

	if r.DLSEXPRESSION != nil && *r.DLSEXPRESSION != "" {
		fraud.DLSEXPRESSION = *r.DLSEXPRESSION
	}

	if r.ENABLED != nil {
		fraud.ENABLED = *r.ENABLED
	}

	if r.PRIORITY != nil {
		fraud.PRIORITY = *r.PRIORITY
	}

	fraud.UpdatedAt = time.Now()

	err = s.fraudRepo.Update(ctx, fraud)
	if err != nil {
		return nil, fmt.Errorf("failed to update fraud: %w", err)
	}

	return fraud, nil
}

func (s *FraudService) ListFraud(ctx context.Context, page, size int) ([]*fraud.Fraud, int, error) {
	if page < 0 {
		return nil, 0, fmt.Errorf("page must be non-negative")
	}

	if size < 1 {
		size = 20
	}

	offset := page * size

	frauds, err := s.fraudRepo.List(ctx, offset, size)

	if err != nil {
		return nil, 0, fmt.Errorf("list frauds: %w", err)
	}

	total, err := s.fraudRepo.COUNT(ctx)

	if err != nil {
		return nil, 0, fmt.Errorf("count frauds: %w", err)
	}

	return frauds, total, nil

}

func (s *FraudService) CheckNameExist(ctx context.Context, name string) (bool, error) {
	return s.fraudRepo.CheckNameExist(ctx, name)
}

func (s *FraudService) ValiteDLS(ctx context.Context, dls string) (*fraud.ValidateDSLResponse, error) {
	if len(dls) < 3 || len(dls) > 2000 {
		return &fraud.ValidateDSLResponse{
			IsValid: false,
			Errors: []fraud.DSLError{{
				Code:    "DSL_PARSE_ERROR",
				Message: "DSL expression must be between 3 and 2000 characters",
			}},
		}, nil
	}

	response := s.validator.Validate(dls)

	return response, nil

} 
