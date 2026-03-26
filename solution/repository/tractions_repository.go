package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"solution/Traction"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type TransactionRepository interface {
	CreateWithResults(ctx context.Context, transaction *Traction.Transaction, ruleResults []Traction.RuleResult) error
	GetByID(ctx context.Context, id uuid.UUID) (*Traction.Transaction, error)
	GetByIDWithResults(ctx context.Context, id uuid.UUID) (*Traction.Transaction, []Traction.RuleResult, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, filters map[string]interface{}) ([]Traction.Transaction, int64, error)
}

type RuleRepository interface {
	GetAllActive(ctx context.Context) ([]Traction.Rule, error)
}

type transactionRepository struct {
	db *sql.DB
}

type ruleRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func NewRuleRepository(db *sql.DB) RuleRepository {
	return &ruleRepository{db: db}
}

func (r *ruleRepository) GetAllActive(ctx context.Context) ([]Traction.Rule, error) {
	query := `SELECT id, name, description, dslexpression AS dsl, priority, enabled, created_at
	          FROM fraud WHERE enabled = true ORDER BY priority, id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query rules: %w", err)
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
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return rules, nil
}

func (r *transactionRepository) CreateWithResults(ctx context.Context, transaction *Traction.Transaction, ruleResults []Traction.RuleResult) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	query := `INSERT INTO transactions (
		id, user_id, amount, currency, status, merchant_id, merchant_category_code,
		timestamp, ip_address, device_id, channel, location, metadata, is_fraud, created_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	locationJSON := []byte("null")
	if transaction.Location != nil {
		locationJSON, err = json.Marshal(transaction.Location)
		if err != nil {
			return fmt.Errorf("failed to marshal location: %w", err)
		}
	}

	metadataJSON := []byte("{}")
	if len(transaction.Metadata) > 0 {
		metadataJSON = transaction.Metadata
	}

	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, query,
		transaction.ID,
		transaction.UserID,
		transaction.Amount,
		transaction.Currency,
		transaction.Status,
		transaction.MerchantID,
		transaction.MerchantCategoryCode,
		transaction.Timestamp,
		transaction.IPAddress,
		transaction.DeviceID,
		transaction.Channel,
		locationJSON,
		metadataJSON,
		transaction.IsFraud,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	for _, result := range ruleResults {
		insertRuleQuery := `INSERT INTO transaction_rule_results (id, transaction_id, rule_id, matched, description, created_at)
		                    VALUES ($1, $2, $3, $4, $5, $6)`
		_, err := tx.ExecContext(ctx, insertRuleQuery,
			uuid.New(),
			transaction.ID,
			result.RuleID,
			result.Matched,
			result.Description,
			now,
		)
		if err != nil {
			return fmt.Errorf("failed to insert rule result: %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (r *transactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*Traction.Transaction, error) {
	query := `SELECT 
		id, user_id, amount, currency, status, merchant_id, merchant_category_code,
		timestamp, ip_address, device_id, channel, location, metadata, is_fraud, created_at, 
		FROM transactions WHERE id = $1`

	var tx Traction.Transaction
	var channelStr sql.NullString
	var locationJSON []byte
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tx.ID,
		&tx.UserID,
		&tx.Amount,
		&tx.Currency,
		&tx.Status,
		&tx.MerchantID,
		&tx.MerchantCategoryCode,
		&tx.Timestamp,
		&tx.IPAddress,
		&tx.DeviceID,
		&channelStr,
		&locationJSON,
		&metadataJSON,
		&tx.IsFraud,
		&tx.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to scan transaction: %w", err)
	}

	if channelStr.Valid {
		ch := channelStr.String
		tx.Channel = &ch
	}

	if len(locationJSON) > 0 && string(locationJSON) != "null" {
		var loc Traction.Location
		if err := json.Unmarshal(locationJSON, &loc); err == nil {
			tx.Location = &loc
		}
	}

	tx.Metadata = metadataJSON

	return &tx, nil
}

func (r *transactionRepository) GetByIDWithResults(ctx context.Context, id uuid.UUID) (*Traction.Transaction, []Traction.RuleResult, error) {
	transaction, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	query := `
		SELECT 
			trr.rule_id,
			r.name,
			r.priority,
			r.enabled,
			trr.matched,
			trr.description
		FROM transaction_rule_results trr
		JOIN fraud r ON trr.rule_id = r.id
		WHERE trr.transaction_id = $1
		ORDER BY r.priority, r.id
	`

	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return transaction, nil, fmt.Errorf("failed to query rule results: %w", err)
	}
	defer rows.Close()

	var ruleResults []Traction.RuleResult
	for rows.Next() {
		var rr Traction.RuleResult
		err := rows.Scan(&rr.RuleID, &rr.RuleName, &rr.Priority, &rr.Enabled, &rr.Matched, &rr.Description)
		if err != nil {
			continue
		}
		ruleResults = append(ruleResults, rr)
	}

	return transaction, ruleResults, nil
}

func (r *transactionRepository) GetByUserID(ctx context.Context, userID uuid.UUID, filters map[string]interface{}) ([]Traction.Transaction, int64, error) {
	baseQuery := `SELECT 
		id, user_id, amount, currency, status, merchant_id, merchant_category_code,
		timestamp, ip_address, device_id, channel, location, metadata, is_fraud, created_at
		FROM transactions WHERE user_id = $1`

	countQuery := `SELECT COUNT(*) FROM transactions WHERE user_id = $1`

	args := []interface{}{userID}
	argIndex := 2

	if status, ok := filters["status"]; ok {
		if s, ok := status.(string); ok {
			baseQuery += fmt.Sprintf(" AND status = $%d", argIndex)
			countQuery += fmt.Sprintf(" AND status = $%d", argIndex)
			args = append(args, s)
			argIndex++
		}
	}

	if isFraud, ok := filters["isFraud"]; ok {
		if b, ok := isFraud.(bool); ok {
			baseQuery += fmt.Sprintf(" AND is_fraud = $%d", argIndex)
			countQuery += fmt.Sprintf(" AND is_fraud = $%d", argIndex)
			args = append(args, b)
			argIndex++
		}
	}

	if from, ok := filters["from"]; ok {
		if t, ok := from.(time.Time); ok {
			baseQuery += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
			countQuery += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
			args = append(args, t)
			argIndex++
		}
	}

	if to, ok := filters["to"]; ok {
		if t, ok := to.(time.Time); ok {
			baseQuery += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
			countQuery += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
			args = append(args, t)
			argIndex++
		}
	}

	baseQuery += " ORDER BY timestamp DESC"

	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args[:argIndex-1]...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	if page, ok := filters["page"]; ok {
		if size, ok2 := filters["size"]; ok2 {
			pageInt, _ := toInt(page)
			sizeInt, _ := toInt(size)
			if sizeInt > 0 {
				offset := pageInt * sizeInt
				baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
				args = append(args, sizeInt, offset)
				argIndex += 2
			}
		}
	}

	rows, err := r.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []Traction.Transaction
	for rows.Next() {
		var tx Traction.Transaction
		var channelStr sql.NullString
		var locationJSON []byte
		var metadataJSON []byte

		err := rows.Scan(
			&tx.ID,
			&tx.UserID,
			&tx.Amount,
			&tx.Currency,
			&tx.Status,
			&tx.MerchantID,
			&tx.MerchantCategoryCode,
			&tx.Timestamp,
			&tx.IPAddress,
			&tx.DeviceID,
			&channelStr,
			&locationJSON,
			&metadataJSON,
			&tx.IsFraud,
			&tx.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan row: %w", err)
		}

		if channelStr.Valid {
			ch := channelStr.String
			tx.Channel = &ch
		}

		if len(locationJSON) > 0 && string(locationJSON) != "null" {
			var loc Traction.Location
			if err := json.Unmarshal(locationJSON, &loc); err == nil {
				tx.Location = &loc
			}
		}

		tx.Metadata = metadataJSON
		transactions = append(transactions, tx)
	}

	return transactions, total, nil
}

func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case float64:
		return int(val), true
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i, true
		}
	}
	return 0, false
}
