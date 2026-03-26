package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	fraud "solution/Fraud"
	"solution/Traction"
	"time"
)

type FraudRepository interface {
	Create(ctx context.Context, fraud *fraud.Fraud) error
	Update(ctx context.Context, fraud *fraud.Fraud) error
	GetBYID(ctx context.Context, id string) (*fraud.Fraud, error)
	List(ctx context.Context, page, limit int) ([]*fraud.Fraud, error)
	CheckNameExist(ctx context.Context, name string) (bool, error)
	DELETE(ctx context.Context, id string) error
	COUNT(ctx context.Context) (int, error)
	GetByName(ctx context.Context, name string) (*Traction.Rule, error)
	GetAllActive(ctx context.Context) ([]Traction.Rule, error)
}

type fraudRepo struct {
	db *sql.DB
}

func NewFraudRepository(db *sql.DB) FraudRepository {
	return &fraudRepo{db: db}
}

func (r *fraudRepo) Create(ctx context.Context, fraud *fraud.Fraud) error {
	query := `INSERT INTO fraud (
	id, name, description, dslexpression, enabled, priority, created_At, updated_At)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8) `
	_, err := r.db.ExecContext(ctx, query,
		fraud.ID, fraud.NAME, fraud.DESCRIPTION, fraud.DLSEXPRESSION, fraud.ENABLED, fraud.PRIORITY,
		fraud.CreatedAt, fraud.UpdatedAt,
	)

	if err != nil {
		return err
	}

	return nil
}
func (r *fraudRepo) GetBYID(ctx context.Context, id string) (*fraud.Fraud, error) {

	query := `SELECT id, name, description, dslexpression, enabled, priority, 
		created_at, updated_at  
	FROM fraud
	WHERE id = $1`

	var f fraud.Fraud
	var createdAt, updatedAt time.Time

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&f.ID, &f.NAME, &f.DESCRIPTION, &f.DLSEXPRESSION, &f.ENABLED, &f.PRIORITY,
		&createdAt, &updatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("fraud not found")
		}
		return nil, err
	}

	f.CreatedAt = createdAt
	f.UpdatedAt = updatedAt

	return &f, nil
}

func (r *fraudRepo) Update(ctx context.Context, fraud *fraud.Fraud) error {
	query := `UPDATE fraud
	SET name = $2, description = $3, dslexpression = $4, enabled = $5, priority = $6, created_At = $7, updated_At = $8
	WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		fraud.ID, fraud.NAME, fraud.DESCRIPTION, fraud.DLSEXPRESSION, fraud.ENABLED, fraud.PRIORITY,
		fraud.CreatedAt, fraud.UpdatedAt,
	)

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("No update")
	}

	return nil
}

func (r *fraudRepo) List(ctx context.Context, offset, limit int) ([]*fraud.Fraud, error) {

	query := `SELECT id, name, description, dslexpression, enabled, priority, 
		created_at, updated_at 
	FROM fraud
	ORDER BY created_at DESC
	LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var frauds []*fraud.Fraud

	for rows.Next() {
		var f fraud.Fraud
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&f.ID, &f.NAME, &f.DESCRIPTION, &f.DLSEXPRESSION,
			&f.ENABLED, &f.PRIORITY, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		f.CreatedAt = createdAt
		f.UpdatedAt = updatedAt
		frauds = append(frauds, &f)
	}

	return frauds, nil
}
func (r *fraudRepo) DELETE(ctx context.Context, id string) error {
	query := `UPDATE fraud SET enable = false, updated_at = $2 WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id, time.Now())

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()

	if rows == 0 {
		return fmt.Errorf("No delete fraud")
	}

	return nil
}

func (r *fraudRepo) CheckNameExist(ctx context.Context, name string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM fraud WHERE name = $1)`

	var exists bool

	err := r.db.QueryRowContext(ctx, query, name).Scan(&exists)

	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *fraudRepo) COUNT(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM fraud`

	var count int

	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count fraud: %w", err)
	}

	return count, nil
}

func (r *fraudRepo) GetByName(ctx context.Context, name string) (*Traction.Rule, error) {
	query := `
		SELECT id, name, description, dsl, priority, enabled, created_at, updated_at
		FROM fraud_rules
		WHERE name = $1
		LIMIT 1`

	var rule Traction.Rule
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&rule.ID,
		&rule.Name,
		&rule.Description,
		&rule.DSL,
		&rule.Priority,
		&rule.Enabled,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get rule by name: %w", err)
	}

	return &rule, nil
}
func (r *fraudRepo) GetAllActive(ctx context.Context) ([]Traction.Rule, error) {
	query := `SELECT id, name, description, dsl, priority, enabled, created_at, updated_at 
	          FROM fraud WHERE enabled = true ORDER BY priority, id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active rules: %w", err)
	}
	defer rows.Close()

	var rules []Traction.Rule
	for rows.Next() {
		var rule Traction.Rule
		err := rows.Scan(
			&rule.ID,
			&rule.Name,
			&rule.Description,
			&rule.DSL,
			&rule.Priority,
			&rule.Enabled,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, nil
}
