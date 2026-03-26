// настройки приложения

package config

import (
	"fmt"
	"os"
)

// структура конфига
type Config struct {
	AdminEmail    string
	AdminUsername string
	AdminPassword string
	JWTsecret     string
	DBhost        string
	DBport        string
	DBname        string
	DBuser        string
	DBpassword    string
}

func (c *Config) GetDatabaseURL() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBhost, c.DBport, c.DBuser, c.DBpassword, c.DBname)
}

// создаем и возвращаем данныые для стркутуры Config
func Load() *Config {
	return &Config{
		AdminEmail:    getEnv("ADMIN_EMAIL", "adminexp@gmail.com"),
		AdminUsername: getEnv("ADMIN_USERNAME", "Admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "ADMIN!@!#@!"),
		JWTsecret:     getEnv("JWT_SECRET", "SECRET"),
		DBhost:        getEnv("DB_HOST", "localhost"),
		DBport:        getEnv("DB_PORT", "5432"),
		DBname:        getEnv("DB_NAME", "postgres"),
		DBuser:        getEnv("DB_USER", "postgres"),
		DBpassword:    getEnv("DB_PASSWORD", "12345678"),
	}
}

// проверка наличие переменного окружения
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
