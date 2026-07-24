# internal/services/ Directory Documentation

Services represent the core business logic layer of the application. They sit between the controllers (HTTP handlers) and the database (SQLC queries / storage).

By decoupling business logic from HTTP mechanics, the application becomes more modular, testable, and maintainable.

## Role of Services
1. **Business Rules**: Execute business validation, coordinate multi-step DB transactions, and apply calculations or domain-specific rules.
2. **Third-Party Integrations**: Interact with external APIs, email service providers, or message queues.
3. **Transaction Coordination**: Manage database transactions across multiple SQLC queries when required.
4. **Caching Orchestration**: Combine database queries with Redis caching logic (e.g., Cache-Aside pattern).

## Structure Example
A typical service is defined using interfaces to facilitate dependency injection and testing/mocking:

```go
package services

import (
	"context"
	
	"github.com/CodeChefVIT/go-backend-template/internal/db"
	"github.com/CodeChefVIT/go-backend-template/internal/dto"
)

type UserService interface {
	GetUserByID(ctx context.Context, id int32) (*dto.UserResponse, error)
	CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserResponse, error)
}

type userService struct {
	queries *db.Queries
}

func NewUserService(queries *db.Queries) UserService {
	return &userService{
		queries: queries,
	}
}

func (s *userService) GetUserByID(ctx context.Context, id int32) (*dto.UserResponse, error) {
	user, err := s.queries.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return dto.ToUserResponse(user), nil
}

func (s *userService) CreateUser(ctx context.Context, req *dto.CreateUserRequest) (*dto.UserResponse, error) {
	// Business logic and validation goes here
	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Username: req.Username,
		Email:    req.Email,
	})
	if err != nil {
		return nil, err
	}
	return dto.ToUserResponse(user), nil
}
```

## Design Rules
1. **Decoupled from HTTP**: Services must not import `github.com/labstack/echo/v4` or deal with HTTP status codes/cookies. Use standard Go types, custom error types, or DTOs.
2. **Context Propagation**: Always accept `context.Context` as the first argument in service functions to ensure database timeouts and request cancellations propagate properly.
3. **Interface-Based Design**: Define services as Go interfaces so they can be mocked in unit tests for controllers.
