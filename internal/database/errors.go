package database

import "errors"

var (
	// ErrRecordNotFound is returned when a record is not found
	ErrRecordNotFound = errors.New("record not found")
	
	// ErrInvalidData is returned when data is invalid
	ErrInvalidData = errors.New("invalid data")
	
	// ErrDatabaseClosed is returned when the database is closed
	ErrDatabaseClosed = errors.New("database is closed")
)
