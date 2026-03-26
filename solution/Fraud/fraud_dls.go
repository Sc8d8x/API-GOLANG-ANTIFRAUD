package fraud

type ValidateDSLRequest struct {
	DSLExpression string `json:"dslExpression" binding:"required,min=3,max=2000"`
}

type ValidateDSLResponse struct {
	IsValid              bool       `json:"isValid"`
	NormalizedExpression *string    `json:"normalizedExpression,omitempty"`
	Errors               []DSLError `json:"errors"`
}

type DSLError struct {
	Code     string  `json:"code"`
	Message  string  `json:"message"`
	Position *int    `json:"position,omitempty"`
	Near     *string `json:"near,omitempty"`
}
