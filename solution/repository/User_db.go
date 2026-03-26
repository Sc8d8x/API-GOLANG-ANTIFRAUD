package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"solution/User"
	"time"
)

// interface для функций
type UserRepository interface {
	Create(ctx context.Context, user *User.User) error
	GetByID(ctx context.Context, id string) (*User.User, error)
	GetByEmail(ctx context.Context, email string) (*User.User, error)
	Update(ctx context.Context, user *User.User) error
	List(ctx context.Context, page, limit int) ([]*User.User, error)
	Count(ctx context.Context) (int, error)
	CheckEmailExist(ctx context.Context, email string) (bool, error)
	Delete(ctx context.Context, id string) error
}

// для работы с базой данных
type userRepo struct {
	db *sql.DB
}

// создаем новый репоситорий
func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) Create(ctx context.Context, user *User.User) error {
	query := `INSERT INTO users (
            id, email, password_hash, full_name, age, region, 
            gender, marital_status, role, is_active, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := r.db.ExecContext(ctx, query,
		user.ID, user.GMAIL, user.PASSWORD, user.FULLNAME, user.AGE,
		user.REGION, user.GENDER, user.MaritalStatus, user.ROLE, user.IsActive,
		user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *userRepo) GetByID(ctx context.Context, id string) (*User.User, error) {
	query := `SELECT id, email, password_hash, full_name, age, region, 
               gender, marital_status, role, is_active, created_at, updated_at
        FROM users 
        WHERE id = $1`

	var user User.User

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.GMAIL, &user.PASSWORD, &user.FULLNAME, &user.AGE,
		&user.REGION, &user.GENDER, &user.MaritalStatus, &user.ROLE, &user.IsActive,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	return &user, nil
}

// возвращаем пользователя по email
func (r *userRepo) GetByEmail(ctx context.Context, email string) (*User.User, error) {
	query := `SELECT id, email, password_hash, full_name, age, region, 
               gender, marital_status, role, is_active, created_at, updated_at
        FROM users 
        WHERE email = $1`

	var user User.User

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.GMAIL, &user.PASSWORD, &user.FULLNAME, &user.AGE,
		&user.REGION, &user.GENDER, &user.MaritalStatus, &user.ROLE, &user.IsActive,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	return &user, nil

}

// обновляем пользователя
func (r *userRepo) Update(ctx context.Context, user *User.User) error {

	query := `
        UPDATE users 
        SET email = $2, password_hash = $3, full_name = $4, 
            age = $5, region = $6, gender = $7, marital_status = $8,
            role = $9, is_active = $10, updated_at = $11
        WHERE id = $1
    `
	result, err := r.db.ExecContext(ctx, query,
		user.ID,            // $1 - WHERE id = $1
		user.GMAIL,         // $2 - email = $2 (важно: EMAIL, а не GMAIL!)
		user.PASSWORD,      // $3 - password_hash = $3
		user.FULLNAME,      // $4 - full_name = $4
		user.AGE,           // $5 - age = $5
		user.REGION,        // $6 - region = $6
		user.GENDER,        // $7 - gender = $7
		user.MaritalStatus, // $8 - marital_status = $8
		user.ROLE,          // $9 - role = $9
		user.IsActive,      // $10 - is_active = $10
		user.UpdatedAt,     // $11 - updated_at = $11
	)

	if err != nil {
		fmt.Printf("Ошибка выполнения запроса: %v\n", err)
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("No update")
	}

	fmt.Printf("Обновлено %d записей\n", rows)
	return nil
}

func (r *userRepo) List(ctx context.Context, offset, limit int) ([]*User.User, error) {
	query := `SELECT id, email, password_hash, full_name, age, region, 
               gender, marital_status, role, is_active, created_at, updated_at
        FROM users 
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User.User

	for rows.Next() {
		var user User.User
		err := rows.Scan(&user.ID, &user.GMAIL, &user.PASSWORD, &user.FULLNAME, &user.AGE,
			&user.REGION, &user.GENDER, &user.MaritalStatus, &user.ROLE, &user.IsActive,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, nil
}

// возвращаем количество пользователей
func (r *userRepo) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}

	return count, nil
}

// проверка Email
func (r *userRepo) CheckEmailExist(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`

	var exists bool

	err := r.db.QueryRowContext(ctx, query, email).Scan(&exists)

	if err != nil {
		return false, err
	}
	return exists, nil
}

// удаляем пользователя
func (r *userRepo) Delete(ctx context.Context, id string) error {
	query := `UPDATE users SET is_active = false, updated_at = $2 WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id, time.Now())

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()

	if rows == 0 {
		return fmt.Errorf("No delete account")
	}

	return nil
}
