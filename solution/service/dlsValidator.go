package service

import (
	fraud "solution/Fraud"
	"strings"
)

type DSLValidator struct{}

func NewDSLValidator() *DSLValidator {
	return &DSLValidator{}
}

func (v *DSLValidator) Validate(dsl string) *fraud.ValidateDSLResponse {
	response := &fraud.ValidateDSLResponse{
		IsValid: true,
		Errors:  []fraud.DSLError{},
	}

	normalized := normalizeExpression(dsl)
	response.NormalizedExpression = &normalized

	if errs := v.validateBasicSyntax(dsl); len(errs) > 0 {
		response.IsValid = false
		response.Errors = append(response.Errors, errs...)
		return response
	}

	if errs := v.validateFieldsAndOperators(dsl); len(errs) > 0 {
		response.IsValid = false
		response.Errors = append(response.Errors, errs...)
	}

	return response
}

func normalizeExpression(dsl string) string {
	dsl = strings.TrimSpace(dsl)

	dsl = strings.ReplaceAll(dsl, "(", "")
	dsl = strings.ReplaceAll(dsl, ")", "")

	dsl = strings.ReplaceAll(dsl, " and ", " AND ")
	dsl = strings.ReplaceAll(dsl, " or ", " OR ")

	dsl = strings.Join(strings.Fields(dsl), " ")

	return dsl
}

func (v *DSLValidator) validateBasicSyntax(dsl string) []fraud.DSLError {
	var errors []fraud.DSLError

	if strings.Contains(dsl, "()") {
		pos := strings.Index(dsl, "()")
		errors = append(errors, fraud.DSLError{
			Code:     "DSL_PARSE_ERROR",
			Message:  "Empty parentheses",
			Position: &pos,
			Near:     func() *string { s := "()"; return &s }(),
		})
	}

	operators := []string{">>", "<<", "==", "!=", ">=", "<=", "&&", "||"}
	for _, op := range operators {
		if strings.Contains(dsl, op) {

			continue
		}
	}

	openCount := strings.Count(dsl, "(")
	closeCount := strings.Count(dsl, ")")
	if openCount != closeCount {
		errors = append(errors, fraud.DSLError{
			Code:    "DSL_PARSE_ERROR",
			Message: "Mismatched parentheses",
		})
	}

	return errors
}

func (v *DSLValidator) validateFieldsAndOperators(dsl string) []fraud.DSLError {
	var errors []fraud.DSLError

	allowedFields := map[string]string{
		"amount":   "number",
		"currency": "string",
		"country":  "string",
	}

	numberOperators := []string{">", "<", ">=", "<=", "=", "!="}

	stringOperators := []string{"=", "!="}

	words := strings.Fields(dsl)
	for i, word := range words {
		if fieldType, exists := allowedFields[word]; exists {

			if i+1 < len(words) {
				operator := words[i+1]

				if fieldType == "number" {
					if !contains(numberOperators, operator) {
						pos := strings.Index(dsl, word) + len(word)
						errors = append(errors, fraud.DSLError{
							Code:     "DSL_INVALID_OPERATOR",
							Message:  "Invalid operator for numeric field",
							Position: &pos,
							Near:     &operator,
						})
					}
				} else if fieldType == "string" {
					if !contains(stringOperators, operator) {
						pos := strings.Index(dsl, word) + len(word)
						errors = append(errors, fraud.DSLError{
							Code:     "DSL_INVALID_OPERATOR",
							Message:  "Invalid operator for string field",
							Position: &pos,
							Near:     &operator,
						})
					}
				}
			}
		}
	}

	return errors
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
