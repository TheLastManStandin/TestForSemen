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

// argSchema is the part of a tool's declared JSON schema that is enforced before the tool
// runs, so the model gets an error instead of a silently ignored or defaulted field.
// It is deliberately not a full JSON Schema validator: value types are checked when the
// arguments are decoded, which tolerates numbers sent as strings and back ("id": "12",
// "amount": 350) — a common model habit that a strict "type" check would turn into errors.
// Numeric bounds (limit 1..50) are clamped by the tools.
type argSchema struct {
	Properties map[string]struct {
		Enum []string `json:"enum"`
	} `json:"properties"`
	Required []string `json:"required"`
}

// compileArgSchemas extracts the enforced part of each tool's parameters.
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

// normalize rejects undeclared fields (additionalProperties: false), missing or null required
// fields and values outside an enum, and returns the arguments with «not set» values removed:
// null in an optional field and "" in an optional enum field, which models often send for
// fields they do not use. Other empty strings are values ("comment": "" clears a comment).
func (s argSchema) normalize(raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, &argError{err: err}
	}
	for _, name := range slices.Sorted(maps.Keys(fields)) {
		p, ok := s.Properties[name]
		if !ok {
			return nil, &argError{err: fmt.Errorf("неизвестное поле %q", name)}
		}
		optional := !slices.Contains(s.Required, name)
		v := bytes.TrimSpace(fields[name])
		var str string
		isEnumStr := len(p.Enum) > 0 && json.Unmarshal(v, &str) == nil
		switch {
		case optional && (string(v) == "null" || isEnumStr && strings.TrimSpace(str) == ""):
			delete(fields, name)
		case isEnumStr:
			i := slices.IndexFunc(p.Enum, func(e string) bool { return strings.EqualFold(e, strings.TrimSpace(str)) })
			if i < 0 {
				return nil, &argError{err: fmt.Errorf("поле %q: недопустимое значение %q, ожидается одно из: %s", name, str, strings.Join(p.Enum, ", "))}
			}
			fields[name], _ = json.Marshal(p.Enum[i]) // canonical spelling for the tool
		}
	}
	for _, name := range s.Required {
		if v, ok := fields[name]; !ok || string(bytes.TrimSpace(v)) == "null" {
			return nil, &argError{err: fmt.Errorf("нет обязательного поля %q", name)}
		}
	}
	return json.Marshal(fields)
}
