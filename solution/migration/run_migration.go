package migration

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

var migrationSQL string
var fraudMigrationSQL string
var tractionMigrationSQL string

func init() {
	initUser()
	initFraud()
	initTractions()
}

func initUser() {
	content, err := os.ReadFile("migrations/migration.sql")
	if err != nil {
		// файл не найден значит создаем  в go
		migrationSQL = `CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(254) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			full_name VARCHAR(200) NOT NULL,
			age INTEGER CHECK (age >= 18 AND age <= 120),
			region VARCHAR(55),
			gender VARCHAR(10) CHECK (gender IN ('MALE', 'FEMALE')),
			marital_status VARCHAR(155) CHECK (marital_status IN ('SINGLE', 'MARRIED', 'DIVORCED', 'WIDOWED')),
			role VARCHAR(10) NOT NULL DEFAULT 'USER' CHECK (role IN ('USER', 'ADMIN')),
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
		CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);
		CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);`
		log.Println("Using built-in migration SQL for users")
	} else {
		migrationSQL = string(content)
		log.Println("Loaded migration from file for users")
	}
}

func initFraud() {
	content, err := os.ReadFile("migrations/fraud_migration.sql")
	if err != nil {

		fraudMigrationSQL = `CREATE TABLE IF NOT EXISTS fraud (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(254) UNIQUE NOT NULL,
			description TEXT,
			dslexpression TEXT NOT NULL,
			priority INTEGER NOT NULL CHECK (priority >= 1 AND priority <= 100),
			enabled BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_fraud_name ON fraud(name);
		CREATE INDEX IF NOT EXISTS idx_fraud_enabled ON fraud(enabled);
		CREATE INDEX IF NOT EXISTS idx_fraud_priority ON fraud(priority);`
		log.Println("Using built-in migration SQL for fraud")
	} else {
		fraudMigrationSQL = string(content)
		log.Println("Loaded migration from file for fraud")
	}
}

func initTractions() {
	content, err := os.ReadFile("migrations/fraud_migration.sql")
	if err != nil {
		tractionMigrationSQL = `CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    amount DECIMAL(12,2) NOT NULL CHECK (amount >= 0.01 AND amount <= 999999999.99),
    currency CHAR(3) NOT NULL,
    status VARCHAR(10) NOT NULL CHECK (status IN ('APPROVED', 'DECLINED')),
    merchant_id VARCHAR(64),
    merchant_category_code CHAR(4),
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    ip_address VARCHAR(64),
    device_id VARCHAR(128),
    channel VARCHAR(10) CHECK (channel IN ('WEB', 'MOBILE', 'POS', 'OTHER')),
    location JSONB,
    metadata JSONB DEFAULT '{}',
    is_fraud BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions(user_id);
	CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
	CREATE INDEX IF NOT EXISTS idx_transactions_is_fraud ON transactions(is_fraud);


	CREATE TABLE IF NOT EXISTS transaction_rule_results (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
		rule_id UUID NOT NULL REFERENCES fraud(id),
		matched BOOLEAN NOT NULL,
		description TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_transaction_rule_results_transaction_id ON transaction_rule_results(transaction_id);
	CREATE INDEX IF NOT EXISTS idx_transaction_rule_results_rule_id ON transaction_rule_results(rule_id);
	`
		log.Println("Using built-in migration SQL for tractions")
	} else {
		tractionMigrationSQL = string(content)
		log.Println("Loaded migration from file for tractions")
	}
}

func CreateBase(db *sql.DB) error {
	if migrationSQL == "" || fraudMigrationSQL == "" || tractionMigrationSQL == "" {
		return fmt.Errorf("no migration SQL found")
	}

	_, err := db.Exec(migrationSQL)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	_, err = db.Exec(fraudMigrationSQL)
	if err != nil {
		return fmt.Errorf("failed to create fraud table: %w", err)
	}

	_, err = db.Exec(tractionMigrationSQL)
	if err != nil {
		return fmt.Errorf("failed to create tractinsL: %w", err)
	}

	return nil
}
