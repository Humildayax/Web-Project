package dto

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	once     sync.Once
	instance *validator.Validate
)

// Validator devuelve un singleton de *validator.Validate.
// Usa el nombre del tag `json` al reportar errores (no el nombre del campo Go).
func Validator() *validator.Validate {
	once.Do(func() {
		instance = validator.New(validator.WithRequiredStructEnabled())
		instance.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	})
	return instance
}

// FormatValidationError convierte el error de validator a un mensaje legible en español.
func FormatValidationError(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err.Error()
	}
	msgs := make([]string, 0, len(ve))
	for _, fe := range ve {
		msgs = append(msgs, fmt.Sprintf("%s %s", fe.Field(), ruleMessage(fe)))
	}
	return strings.Join(msgs, "; ")
}

func ruleMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "es obligatorio"
	case "min":
		return fmt.Sprintf("debe tener al menos %s caracteres", fe.Param())
	case "max":
		return fmt.Sprintf("debe tener máximo %s caracteres", fe.Param())
	case "email":
		return "no es un email válido"
	case "url":
		return "debe ser una URL válida"
	case "omitempty":
		return ""
	default:
		return fmt.Sprintf("falló la regla '%s'", fe.Tag())
	}
}
