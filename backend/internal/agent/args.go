package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

// flexString accepts a JSON string or number: models sometimes send 350 instead of "350".
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return errors.New("ожидается строка или число")
	}
	*f = flexString(n.String())
	return nil
}

// flexInt accepts a JSON number or a numeric string.
type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return fmt.Errorf("ожидается целое число, получено %q", s)
		}
		*f = flexInt(v)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return errors.New("ожидается целое число")
	}
	v, err := n.Int64()
	if err != nil {
		return fmt.Errorf("ожидается целое число, получено %s", n)
	}
	*f = flexInt(v)
	return nil
}

// argError marks malformed tool arguments.
type argError struct{ err error }

func (e *argError) Error() string {
	return "некорректные аргументы: " + e.err.Error()
}

func decodeArgs(raw []byte, v any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return &argError{err: err}
	}
	return nil
}

// rangeArgs is embedded by tools that accept a period.
type rangeArgs struct {
	Period string `json:"period"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// argSchema is the part of a tool's declared JSON schema that is checked before the tool
// runs, so the model gets an error instead of a silently ignored or defaulted field.
// Value types are checked when the arguments are decoded.
type argSchema struct {
	Properties map[string]struct {
		Enum []string `json:"enum"`
	} `json:"properties"`
	Required []string `json:"required"`
}

// compileArgSchemas extracts the checked part of each tool's parameters.
func compileArgSchemas(tools []llm.Tool) map[string]argSchema {
	out := make(map[string]argSchema, len(tools))
	for _, t := range tools {
		var s argSchema
		if err := json.Unmarshal(t.Function.Parameters, &s); err != nil {
			panic(fmt.Sprintf("tool %s: %v", t.Function.Name, err))
		}
		out[t.Function.Name] = s
	}
	return out
}

// check rejects undeclared fields (additionalProperties: false), missing or null required
// fields and values outside an enum. An empty string or null in an optional field means
// «not set», as models often send them for fields they do not use.
func (s argSchema) check(raw []byte) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return &argError{err: err}
	}
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		p, ok := s.Properties[name]
		if !ok {
			return &argError{err: fmt.Errorf("неизвестное поле %q", name)}
		}
		var v string
		if len(p.Enum) == 0 || json.Unmarshal(fields[name], &v) != nil || v == "" || slices.Contains(p.Enum, v) {
			continue
		}
		return &argError{err: fmt.Errorf("поле %q: недопустимое значение %q, ожидается одно из: %s", name, v, strings.Join(p.Enum, ", "))}
	}
	for _, name := range s.Required {
		if v, ok := fields[name]; !ok || string(bytes.TrimSpace(v)) == "null" {
			return &argError{err: fmt.Errorf("нет обязательного поля %q", name)}
		}
	}
	return nil
}
