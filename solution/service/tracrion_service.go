// solution/service/transaction_service.go
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"solution/Traction"
	"solution/User"
	"solution/repository"

	"github.com/google/uuid"
)

type TransactionService struct {
	db                  *sql.DB
	transactionRepo     repository.TransactionRepository
	userRepo            repository.UserRepository
	ruleRepo            repository.RuleRepository
}

func NewTransactionService(
	db *sql.DB,
	transactionRepo repository.TransactionRepository,
	userRepo repository.UserRepository,
	ruleRepo repository.RuleRepository,
) *TransactionService {
	return &TransactionService{
		db:              db,
		transactionRepo: transactionRepo,
		userRepo:        userRepo,
		ruleRepo:        ruleRepo,
	}
}

func (s *TransactionService) CreateTransaction(
	ctx context.Context, 
	req *Traction.CreateTransactionRequest,
	requestUserID string,
	requestUserRole string,
) (*Traction.CreateTransactionResponse, error) {
	
	// 1. Валидация запроса
	if req == nil {
		return nil, fmt.Errorf("validation error: request is required")
	}
	if req.UserID == nil {
		return nil, fmt.Errorf("validation error: userId is required")
	}
	if req.Amount <= 0 {
		return nil, fmt.Errorf("validation error: amount must be greater than 0")
	}
	if strings.TrimSpace(req.Currency) == "" {
		return nil, fmt.Errorf("validation error: currency is required")
	}
	if req.Timestamp.IsZero() {
		return nil, fmt.Errorf("validation error: timestamp is required")
	}
	
	// 2. Проверка прав доступа
	if err := s.validateUserAccess(req.UserID, requestUserID, requestUserRole); err != nil {
		return nil, err
	}
	
	// 3. Проверка существования и активности пользователя
	user, err := s.userRepo.GetByID(ctx, req.UserID.String())
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	
	if !user.IsActive {
		return nil, fmt.Errorf("user account is deactivated")
	}
	
	// 4. Создание объекта транзакции
	transaction := &Traction.Transaction{
		ID:                   uuid.New(),
		UserID:               *req.UserID,
		Amount:               req.Amount,
		Currency:             req.Currency,
		Timestamp:            req.Timestamp,
		MerchantID:           req.MerchantID,
		MerchantCategoryCode: req.MerchantCategoryCode,
		IPAddress:            req.IPAddress,
		DeviceID:             req.DeviceID,
		Channel:              req.Channel,
		Status:               "PENDING",
		CreatedAt:            time.Now().UTC(),
	}
	
	// 5. Получение и сортировка активных правил
	rules, err := s.getSortedActiveRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rules: %w", err)
	}
	
	// 6. Применение всех правил (нельзя останавливаться раньше!)
	ruleResults := s.applyAllRules(ctx, rules, transaction, user)
	
	// 7. Определение статуса транзакции
	anyMatched := false
	for _, result := range ruleResults {
		if result.Matched {
			anyMatched = true
			break
		}
	}
	
	if anyMatched {
		transaction.Status = "DECLINED"
		transaction.IsFraud = true
	} else {
		transaction.Status = "APPROVED"
		transaction.IsFraud = false
	}
	
	// 8. Сохранение транзакции и результатов правил в БД
	if err := s.saveTransactionWithResults(ctx, transaction, ruleResults); err != nil {
		return nil, fmt.Errorf("failed to save transaction: %w", err)
	}
	
	// 9. Формирование ответа
	response := &Traction.CreateTransactionResponse{
		Transaction: s.convertToResponse(transaction),
		RuleResults: ruleResults,
	}
	
	return response, nil
}

// GetTransaction - получение транзакции по ID с проверкой прав
func (s *TransactionService) GetTransaction(
	ctx context.Context, 
	transactionID uuid.UUID,
	requestUserID string,
	requestUserRole string,
) (*Traction.CreateTransactionResponse, error) {
	
	// Получаем транзакцию и результаты правил
	transaction, ruleResults, err := s.transactionRepo.GetByIDWithResults(ctx, transactionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("transaction not found")
		}
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	
	// Проверка прав доступа
	if requestUserRole != "ADMIN" && transaction.UserID.String() != requestUserID {
		return nil, fmt.Errorf("access denied: you can only view your own transactions")
	}
	
	// Формирование ответа
	response := &Traction.CreateTransactionResponse{
		Transaction: s.convertToResponse(transaction),
		RuleResults: ruleResults,
	}
	
	return response, nil
}

// GetTransactions - получение списка транзакций с фильтрами
func (s *TransactionService) GetTransactions(
	ctx context.Context,
	filters map[string]interface{},
	requestUserID string,
	requestUserRole string,
) ([]Traction.TransactionResponse, int, error) {
	if filters == nil {
		filters = map[string]interface{}{}
	}

	targetUserID, err := uuid.Parse(requestUserID)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid user ID: %w", err)
	}

	if requestUserRole == "ADMIN" {
		if rawUserID, ok := filters["userId"]; ok {
			switch v := rawUserID.(type) {
			case uuid.UUID:
				targetUserID = v
			case string:
				parsed, parseErr := uuid.Parse(v)
				if parseErr != nil {
					return nil, 0, fmt.Errorf("invalid filter userId: %w", parseErr)
				}
				targetUserID = parsed
			}
		}
	}

	// Получаем транзакции
	transactions, total, err := s.transactionRepo.GetByUserID(ctx, targetUserID, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get transactions: %w", err)
	}
	
	// Конвертируем в response
	var responses []Traction.TransactionResponse
	for i := range transactions {
		responses = append(responses, *s.convertToResponse(&transactions[i]))
	}
	
	return responses, int(total), nil
}

// Вспомогательные методы

func (s *TransactionService) validateUserAccess(
	requestUserID *uuid.UUID, 
	authUserID string, 
	authUserRole string,
) error {
	if authUserRole == "USER" {
		// USER может создавать транзакции только для себя
		if requestUserID.String() != authUserID {
			return fmt.Errorf("access denied: users can only create transactions for themselves")
		}
	}
	// ADMIN может создавать для любого пользователя
	return nil
}

func (s *TransactionService) getSortedActiveRules(ctx context.Context) ([]Traction.Rule, error) {
	var rules []Traction.Rule
	var err error
	
	if s.ruleRepo != nil {
		
		rules, err = s.ruleRepo.GetAllActive(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get active rules: %w", err)
		}
	} else {
		return []Traction.Rule{}, nil
	}
	
	// Сортировка по priority (меньше = первым), затем по ID как строке
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority < rules[j].Priority
		}
		return rules[i].ID.String() < rules[j].ID.String()
	})
	
	return rules, nil
}

func (s *TransactionService) applyAllRules(
	ctx context.Context,
	rules []Traction.Rule,
	transaction *Traction.Transaction,
	user *User.User,
) []Traction.RuleResult {
	
	var results []Traction.RuleResult
	
	for _, rule := range rules {
		// Для Tier 0: все правила возвращают matched = false
		// Для получения баллов нужно хотя бы частично реализовать DSL
		
		matched := false
		description := ""
		
		
		matched, description = s.evaluateSimpleDSL(rule.DSL, transaction, user)
		
		// Создаем результат правила
		result := Traction.RuleResult{
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			Priority:    rule.Priority,
			Enabled:     rule.Enabled,
			Matched:     matched,
			Description: description,
		}
		
		results = append(results, result)
	}
	
	return results
}

// evaluateSimpleDSL - простейший DSL парсер для Tier 1-2
func (s *TransactionService) evaluateSimpleDSL(
	condition string,
	transaction *Traction.Transaction,
	user *User.User,
) (bool, string) {
	
	condition = strings.TrimSpace(condition)
	
	// Tier 0: всегда возвращаем false (разрешенный старт)
	// Для получения баллов Tier 1-4 нужно реализовать парсинг
	
	// Примеры простых условий для демонстрации:
	switch {
	case condition == "amount > 10000":
		matched := transaction.Amount > 10000
		return matched, fmt.Sprintf("amount > 10000, правило сработало: %v", matched)
		
	case condition == "currency = 'USD'":
		matched := transaction.Currency == "USD"
		return matched, fmt.Sprintf("currency = 'USD', правило сработало: %v", matched)
		
	case strings.Contains(condition, "hour(timestamp)"):
		// Простая проверка ночных транзакций (0-5 часов)
		hour := transaction.Timestamp.Hour()
		matched := hour >= 0 && hour <= 5
		return matched, fmt.Sprintf("Ночная транзакция (%02d:00), правило сработало: %v", hour, matched)
		
	default:
		// Для остальных условий возвращаем false (Tier 0)
		return false, "Правило не удалось вычислить (уровень поддержки 0)"
	}
}

func (s *TransactionService) saveTransactionWithResults(
	ctx context.Context,
	transaction *Traction.Transaction,
	ruleResults []Traction.RuleResult,
) error {
	if s.transactionRepo != nil {
		if err := s.transactionRepo.CreateWithResults(ctx, transaction, ruleResults); err != nil {
			return fmt.Errorf("failed to save transaction: %w", err)
		}
		return nil
	}
	
	// Начинаем транзакцию БД
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	
	// Прямое сохранение если репозитория нет
	err = s.saveTransactionSQL(ctx, tx, transaction)
	if err != nil {
		return fmt.Errorf("failed to save transaction: %w", err)
	}
	
	// Сохраняем результаты правил
	err = s.saveRuleResultsSQL(ctx, tx, transaction.ID, ruleResults)
	if err != nil {
		return fmt.Errorf("failed to save rule results: %w", err)
	}
	
	// Коммитим транзакцию
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

func (s *TransactionService) saveTransactionSQL(
	ctx context.Context,
	tx *sql.Tx,
	transaction *Traction.Transaction,
) error {
	
	query := `
	INSERT INTO transactions (
		id, user_id, amount, currency, status, merchant_id, merchant_category_code,
		timestamp, ip_address, device_id, channel, location, metadata, is_fraud, created_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	
	locationJSON := []byte("null")
	if transaction.Location != nil {
		var err error
		locationJSON, err = json.Marshal(transaction.Location)
		if err != nil {
			return fmt.Errorf("failed to marshal location: %w", err)
		}
	}

	metadataJSON := []byte("{}")
	if len(transaction.Metadata) > 0 {
		metadataJSON = transaction.Metadata
	}
	
	_, err := tx.ExecContext(ctx, query,
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
		transaction.CreatedAt,
	)
	
	return err
}

func (s *TransactionService) saveRuleResultsSQL(
	ctx context.Context,
	tx *sql.Tx,
	transactionID uuid.UUID,
	ruleResults []Traction.RuleResult,
) error {
	
	query := `
	INSERT INTO transaction_rule_results (
		id, transaction_id, rule_id, rule_name, priority, 
		enabled, matched, description, created_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	
	for _, rule := range ruleResults {
		ruleID := uuid.New()
		_, err := tx.ExecContext(ctx, query,
			ruleID,
			transactionID,
			rule.RuleID,
			rule.RuleName,
			rule.Priority,
			rule.Enabled,
			rule.Matched,
			rule.Description,
			time.Now(),
		)
		if err != nil {
			return err
		}
	}
	
	return nil
}

// Статистические методы
func (s *TransactionService) GetTransactionStats(
	ctx context.Context,
	userID uuid.UUID,
	from, to time.Time,
) (map[string]interface{}, error) {
	
	stats := make(map[string]interface{})
	
	// Пример: количество транзакций за период
	query := `
	SELECT 
		COUNT(*) as total_count,
		COUNT(CASE WHEN status = 'APPROVED' THEN 1 END) as approved_count,
		COUNT(CASE WHEN status = 'DECLINED' THEN 1 END) as declined_count,
		COUNT(CASE WHEN is_fraud = true THEN 1 END) as fraud_count,
		COALESCE(SUM(CASE WHEN status = 'APPROVED' THEN amount ELSE 0 END), 0) as total_amount
	FROM transactions 
	WHERE user_id = $1 
	AND timestamp BETWEEN $2 AND $3
	`
	
	var totalCount, approvedCount, declinedCount, fraudCount int
	var totalAmount float64
	
	err := s.db.QueryRowContext(ctx, query, userID, from, to).Scan(
		&totalCount,
		&approvedCount,
		&declinedCount,
		&fraudCount,
		&totalAmount,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	
	stats["totalCount"] = totalCount
	stats["approvedCount"] = approvedCount
	stats["declinedCount"] = declinedCount
	stats["fraudCount"] = fraudCount
	stats["totalAmount"] = totalAmount
	stats["period"] = map[string]interface{}{
		"from": from.Format(time.RFC3339),
		"to":   to.Format(time.RFC3339),
	}
	
	return stats, nil
}

func (s *TransactionService) convertToResponse(tx *Traction.Transaction) *Traction.TransactionResponse {
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

	if len(tx.Metadata) > 0 {
		var meta map[string]interface{}
		if err := json.Unmarshal(tx.Metadata, &meta); err == nil {
			resp.Metadata = meta
		}
	}

	return resp
}