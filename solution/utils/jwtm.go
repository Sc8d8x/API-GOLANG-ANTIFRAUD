// файл для создание токенов

package utils

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"sub"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// генерация или создание токена для пароля
func GenerateJWT(userID, role, secret string) (string, int, error) {
	expiris := 3600
	expirisTime := time.Now().Add(time.Duration(expiris) * time.Second)

	claims := &Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirisTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenscript, err := token.SignedString([]byte(secret))
	return tokenscript, expiris, err
}

// проверка токена на валидность
func ValidateJWT(tokenscript, secret string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenscript, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, err
	}

	return claims, err
}
