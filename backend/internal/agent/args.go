package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
