package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"goyco/internal/validation"
)

func ValidationMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" || r.Method == "DELETE" || r.Method == "HEAD" || r.Method == "OPTIONS" {
				next.ServeHTTP(w, r)
				return
			}

			dtoType := GetDTOTypeFromContext(r.Context())
			if dtoType == nil {
				next.ServeHTTP(w, r)
				return
			}

			dto := reflect.New(dtoType).Interface()

			if err := json.NewDecoder(r.Body).Decode(dto); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			if err := validation.ValidateStruct(dto); err != nil {
				var errorMessages []string
				if structErr, ok := err.(*validation.StructValidationError); ok {
					errorMessages = make([]string, len(structErr.Errors))
					for i, fieldError := range structErr.Errors {
						errorMessages[i] = fieldError.Message
					}
				} else {
					errorMessages = []string{err.Error()}
				}

				response := map[string]any{
					"success": false,
					"error":   "Validation failed",
					"details": strings.Join(errorMessages, "; "),
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(response)
				return
			}

			ctx := context.WithValue(r.Context(), validatedDTOKey, dto)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

const DTOTypeKey contextKey = "dto_type"
const validatedDTOKey contextKey = "validated_dto"

func SetDTOTypeInContext(ctx context.Context, dtoType reflect.Type) context.Context {
	return context.WithValue(ctx, DTOTypeKey, dtoType)
}

func GetDTOTypeFromContext(ctx context.Context) reflect.Type {
	if dtoType, ok := ctx.Value(DTOTypeKey).(reflect.Type); ok {
		return dtoType
	}
	return nil
}

func GetValidatedDTOFromContext(ctx context.Context) any {
	return ctx.Value(validatedDTOKey)
}

func SetValidatedDTOInContext(ctx context.Context, dto any) context.Context {
	return context.WithValue(ctx, validatedDTOKey, dto)
}
