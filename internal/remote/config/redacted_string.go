package config

import "fmt"

type RedactedString string

func (r RedactedString) String() string {
	return fmt.Sprintf("<redacted-%d-chars>", len(r))
}

func (r RedactedString) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", r.String())), nil
}

func (r RedactedString) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

func (r RedactedString) MarshalBinary() ([]byte, error) {
	return []byte(r.String()), nil
}
