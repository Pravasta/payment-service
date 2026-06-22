package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSONMap memetakan map[string]any ke kolom JSONB PostgreSQL tanpa dependensi
// tambahan (mengimplementasi driver.Valuer & sql.Scanner).
type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *JSONMap) Scan(src any) error {
	if src == nil {
		*m = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("JSONMap: unsupported scan type")
	}
	return json.Unmarshal(b, m)
}

// GormDataType memberi tahu GORM tipe kolomnya jsonb.
func (JSONMap) GormDataType() string { return "jsonb" }
