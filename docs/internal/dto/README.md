# internal/dto/ Directory Documentation

Data Transfer Objects (DTOs) define the structures of requests and responses exchanged by the API.

## DTO Modules
- `common.go`: Standardized Success and Error formats wrapper for the entire application.

## Validation Annotation Patterns
The project integrates `go-playground/validator/v10` to automate request body parsing validations. When you define your custom structs, configure validation tags:
- `validate:"required"`: Cannot be zero-valued.
- `validate:"email"`: Must conform to email syntax.
- `validate:"min=3"`: Minimum length / value of 3.
- `validate:"max=100"`: Maximum length of 100 characters.

Example structure:
```go
type UserPayload struct {
	Name  string `json:"name" validate:"required,min=3"`
	Email string `json:"email" validate:"required,email"`
}
```
