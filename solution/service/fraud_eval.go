package service

import (
	"context"
	"solution/Traction"

	"github.com/Knetic/govaluate"
	"github.com/google/uuid"
)

type UserContextProvider interface {
	GetUserAgeAndRegion(ctx context.Context, userID uuid.UUID) (age int, region string, err error)
}

func (s *FraudService) EvaluateRule(
	ctx context.Context,
	rule Traction.Rule,
	tx *Traction.Transaction,
	userProvider UserContextProvider,
) (bool, error) {

	if rule.DSL == "" {
		return false, nil
	}

	params := make(map[string]interface{})

	params["transaction.amount"] = tx.Amount
	params["transaction.currency"] = tx.Currency
	params["transaction.status"] = tx.Status
	params["transaction.merchantCategoryCode"] = tx.MerchantCategoryCode
	params["transaction.isFraud"] = tx.IsFraud
	if tx.IPAddress != nil {
		params["transaction.ipAddress"] = *tx.IPAddress
	}
	if tx.DeviceID != nil {
		params["transaction.deviceId"] = *tx.DeviceID
	}
	if tx.Channel != nil {
		params["transaction.channel"] = *tx.Channel
	}

	if userProvider != nil && tx.UserID != uuid.Nil {
		age, region, err := userProvider.GetUserAgeAndRegion(ctx, tx.UserID)
		if err == nil {
			params["user.age"] = age
			params["user.region"] = region
		}

	}

	expr, err := govaluate.NewEvaluableExpression(rule.DSL)
	if err != nil {

		return false, nil
	}

	result, err := expr.Evaluate(params)
	if err != nil {

		return false, nil
	}

	matched, ok := result.(bool)
	if !ok {

		if f, ok := result.(float64); ok {
			matched = f != 0
		} else if i, ok := result.(int); ok {
			matched = i != 0
		} else {
			matched = false
		}
	}

	return matched, nil
}
