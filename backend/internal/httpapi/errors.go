package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

func writeProblem(w http.ResponseWriter, status int, title, detail string, fields []gen.FieldError) {
	p := gen.Problem{Title: title, Status: status}
	if detail != "" {
		p.Detail = &detail
	}
	if len(fields) > 0 {
		p.Errors = &fields
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

// errorHandler maps handler errors to RFC 7807 responses.
func errorHandler(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		var ve *domain.ValidationError
		switch {
		case errors.As(err, &ve):
			writeProblem(w, http.StatusUnprocessableEntity, "Ошибка валидации", ve.Message,
				[]gen.FieldError{{Field: ve.Field, Message: ve.Message}})
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "Не найдено", "", nil)
		case errors.Is(err, domain.ErrConflict):
			writeProblem(w, http.StatusConflict, "Конфликт изменений",
				"Трата была изменена другим пользователем. Обновите данные и повторите.", nil)
		default:
			log.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
			writeProblem(w, http.StatusInternalServerError, "Внутренняя ошибка", "", nil)
		}
	}
}

// badRequest handles malformed parameters and bodies rejected by generated code. Requests are
// validated against the spec first (validate.go), so this is a fallback; the generated error
// text is English, so it is logged and stays out of the user-facing detail.
func badRequest(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		logRejected(log, r, err)
		writeProblem(w, http.StatusBadRequest, "Некорректный запрос", "параметры или тело запроса не соответствуют API", nil)
	}
}

// logRejected records why a request was rejected: the response detail is deliberately generic.
func logRejected(log *slog.Logger, r *http.Request, err error) {
	log.InfoContext(r.Context(), "request rejected", "method", r.Method, "path", r.URL.Path, "err", err)
}
