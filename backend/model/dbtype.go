package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"strings"
)

// StringArray is a custom type for handling PostgreSQL text arrays in GORM.
type StringArray []string

// Scan implements the sql.Scanner interface for deserializing the array.
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = StringArray{}
		return nil
	}

	s, ok := value.(string)
	if !ok {
		return errors.New("failed to scan StringArray")
	}

	s = strings.Trim(s, "{}")
	if s == "" {
		*a = StringArray{}
		return nil
	}

	*a = strings.Split(s, ",")
	return nil
}

// Value implements the driver.Valuer interface for serializing the array.
func (a StringArray) Value() (driver.Value, error) {
	return "{" + strings.Join(a, ",") + "}", nil
}

func (a StringArray) GormDataType() string {
	return "text[]"
}

type Inet net.IP

// Value returns value as a string.
func (ip Inet) Value() (driver.Value, error) {
	return net.IP(ip).String(), nil
}

// Scan scans a string value into Inet.
func (ip *Inet) Scan(value interface{}) error {

	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("can' scan: %s", value)
	}
	*ip = Inet(net.ParseIP(s))
	return nil
}

// GormDataType gorm common data type.
func (Inet) GormDataType() string {
	return "inet"
}
