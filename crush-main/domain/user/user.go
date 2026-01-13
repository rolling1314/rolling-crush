package user

import (
	"context"
	"database/sql"

	"github.com/charmbracelet/crush/store/postgres"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	AvatarURL    sql.NullString
	CreatedAt    int64
	UpdatedAt    int64
}

type Service interface {
	Create(ctx context.Context, username, email, password string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByUsername(ctx context.Context, username string) (User, error)
	Update(ctx context.Context, user User) (User, error)
	UpdatePassword(ctx context.Context, userID, newPassword string) error
	Delete(ctx context.Context, id string) error
	VerifyPassword(ctx context.Context, email, password string) (User, error)
}

type service struct {
	q postgres.Querier
}

func NewService(q postgres.Querier) Service {
	return &service{q: q}
}

func (s *service) Create(ctx context.Context, username, email, password string) (User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}

	dbUser, err := s.q.CreateUser(ctx, postgres.CreateUserParams{
		ID:           uuid.New().String(),
		Username:     username,
		Email:        email,
		PasswordHash: string(hashedPassword),
		AvatarUrl:    sql.NullString{},
	})
	if err != nil {
		return User{}, err
	}

	return s.fromDBItem(dbUser), nil
}

func (s *service) GetByID(ctx context.Context, id string) (User, error) {
	dbUser, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	return s.fromDBItem(dbUser), nil
}

func (s *service) GetByEmail(ctx context.Context, email string) (User, error) {
	dbUser, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, err
	}
	return s.fromDBItem(dbUser), nil
}

func (s *service) GetByUsername(ctx context.Context, username string) (User, error) {
	dbUser, err := s.q.GetUserByUsername(ctx, username)
	if err != nil {
		return User{}, err
	}
	return s.fromDBItem(dbUser), nil
}

func (s *service) Update(ctx context.Context, user User) (User, error) {
	dbUser, err := s.q.UpdateUser(ctx, postgres.UpdateUserParams{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		AvatarUrl: user.AvatarURL,
	})
	if err != nil {
		return User{}, err
	}
	return s.fromDBItem(dbUser), nil
}

func (s *service) UpdatePassword(ctx context.Context, userID, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.q.UpdateUserPassword(ctx, postgres.UpdateUserPasswordParams{
		ID:           userID,
		PasswordHash: string(hashedPassword),
	})
}

func (s *service) Delete(ctx context.Context, id string) error {
	return s.q.DeleteUser(ctx, id)
}

func (s *service) VerifyPassword(ctx context.Context, email, password string) (User, error) {
	dbUser, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(password)); err != nil {
		return User{}, err
	}

	return s.fromDBItem(dbUser), nil
}

func (s *service) fromDBItem(item postgres.User) User {
	return User{
		ID:           item.ID,
		Username:     item.Username,
		Email:        item.Email,
		PasswordHash: item.PasswordHash,
		AvatarURL:    item.AvatarUrl,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

