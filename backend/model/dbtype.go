// Package model defines the data structures and types used throughout the DMARC analyzer.
// It includes both XML models for parsing DMARC reports and database models for storage.
package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"strings"
)

// StringArray is a custom type for handling PostgreSQL text arrays in GORM.
// It implements the necessary interfaces to convert between Go string slices
// and PostgreSQL text[] arrays during database operations.
type StringArray []string

// Scan implements the sql.Scanner interface for deserializing the array.
// This method converts a PostgreSQL text[] array from the database into a Go string slice.
//
// Parameters:
//   - value: The database value to scan (expected to be a string representation of a PostgreSQL array)
//
// Returns:
//   - error: Any error encountered during scanning
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
// This method converts a Go string slice into a PostgreSQL text[] array format for storage.
//
// Returns:
//   - driver.Value: The string representation of the array in PostgreSQL format
//   - error: Any error encountered during conversion
func (a StringArray) Value() (driver.Value, error) {
	return "{" + strings.Join(a, ",") + "}", nil
}

// GormDataType specifies the PostgreSQL data type to use for this custom type.
// This method is used by GORM to determine the database column type when creating tables.
//
// Returns:
//   - string: The PostgreSQL data type name ("text[]")
func (a StringArray) GormDataType() string {
	return "text[]"
}

// Inet is a custom type for handling PostgreSQL inet type in GORM.
// It wraps the standard library's net.IP type to provide database serialization.
type Inet net.IP

// Value returns the IP address as a string for database storage.
// This method implements the driver.Valuer interface for the Inet type.
// If the IP is nil, it returns nil to handle null values in the database.
//
// Returns:
//   - driver.Value: The string representation of the IP address or nil if the IP is nil
//   - error: Any error encountered during conversion
func (ip Inet) Value() (driver.Value, error) {
	if len(ip) == 0 {
		return nil, nil
	}
	return net.IP(ip).String(), nil
}

// Scan converts a database value (string) into an Inet type.
// This method implements the sql.Scanner interface for the Inet type.
// It handles nil values by setting the IP to an empty value.
//
// Parameters:
//   - value: The database value to scan (expected to be a string representation of an IP address or nil)
//
// Returns:
//   - error: Any error encountered during scanning
func (ip *Inet) Scan(value interface{}) error {
	if value == nil {
		*ip = Inet(net.IP{})
		return nil
	}

	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("can't scan: %v", value)
	}
	*ip = Inet(net.ParseIP(s))
	return nil
}

// GormDataType specifies the PostgreSQL data type to use for this custom type.
// This method is used by GORM to determine the database column type when creating tables.
//
// Returns:
//   - string: The PostgreSQL data type name ("inet")
func (Inet) GormDataType() string {
	return "inet"
}
