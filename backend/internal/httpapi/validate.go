package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

// validateRequests rejects requests that do not follow api/openapi.yaml before they reach the
// generated handlers, which decode leniently: unknown fields are ignored and a missing field is
// indistinguishable from its zero value. Paths outside the spec pass through untouched.
func validateRequests(prefix string, next http.Handler) (http.Handler, error) {
	spec, err := gen.GetSpec()
	if err != nil {
		return nil, fmt.Errorf("load openapi spec: %w", err)
	}
	spec.Servers = nil // routes are matched on the path without prefix
	router, err := legacy.NewRouter(spec)
	if err != nil {
		return nil, fmt.Errorf("openapi router: %w", err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, ok := strings.CutPrefix(r.URL.Path, prefix)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		in := r.Clone(r.Context())
		in.URL.Path = path
		route, params, err := router.FindRoute(in)
		var notRouted *routers.RouteError
		if errors.As(err, &notRouted) { // not in the spec (or hand-written, like SSE): the mux decides
			next.ServeHTTP(w, r)
			return
		}
		if err == nil {
			err = singleJSONValue(in)
		}
		if err == nil {
			err = openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{
				Request: in, PathParams: params, Route: route,
				Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
			})
		}
		if err != nil {
			writeRequestProblem(w, err)
			return
		}
		r.Body = in.Body // validation consumed the body and left a fresh reader on the clone
		next.ServeHTTP(w, r)
	}), nil
}

// singleJSONValue rejects a body with data after the JSON value, which decoders ignore.
// It leaves a fresh reader with the same bytes on r.
func singleJSONValue(r *http.Request) error {
	if r.Body == nil || r.Body == http.NoBody {
		return nil
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var v json.RawMessage
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("некорректный JSON: %w", err)
	}
	if err := dec.Decode(&v); !errors.Is(err, io.EOF) {
		return errors.New("некорректный JSON: данные после значения")
	}
	return nil
}

// writeRequestProblem answers a value that breaks the schema with 422 and the offending field
// (a body path or a parameter name), as the panel shows field errors next to the inputs.
// Anything else, such as malformed JSON or an unparsable parameter, is a 400.
func writeRequestProblem(w http.ResponseWriter, err error) {
	var re *openapi3filter.RequestError
	var se *openapi3.SchemaError
	if errors.As(err, &re) && errors.As(err, &se) {
		field, message := describeSchemaError(se)
		if re.Parameter != nil {
			field = re.Parameter.Name
		}
		writeProblem(w, http.StatusUnprocessableEntity, "Ошибка валидации", message, []gen.FieldError{{Field: field, Message: message}})
		return
	}
	writeProblem(w, http.StatusBadRequest, "Некорректный запрос", err.Error(), nil)
}

// describeSchemaError names the field (dot-separated JSON path) and explains the error in Russian.
func describeSchemaError(se *openapi3.SchemaError) (field, message string) {
	path := se.JSONPointer()
	switch se.SchemaField {
	case "properties": // additionalProperties: false; the key is not part of the pointer
		if obj, ok := se.Value.(map[string]any); ok && se.Schema != nil {
			for _, k := range slices.Sorted(maps.Keys(obj)) {
				if _, declared := se.Schema.Properties[k]; !declared {
					path = append(path, k)
					break
				}
			}
		}
		message = "неизвестное поле"
	case "required":
		message = "обязательное поле"
	case "enum":
		var allowed []string
		for _, v := range se.Schema.Enum {
			allowed = append(allowed, fmt.Sprint(v))
		}
		message = "недопустимое значение, ожидается одно из: " + strings.Join(allowed, ", ")
	default:
		message = "недопустимое значение"
	}
	return strings.Join(path, "."), message
}
