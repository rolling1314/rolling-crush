package handler

import "database/sql"

// nullStringToPtr converts sql.NullString to *string
func nullStringToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// ptrToNullString converts *string to sql.NullString
func ptrToNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

// nullInt32ToPtr converts sql.NullInt32 to *int32
func nullInt32ToPtr(ni sql.NullInt32) *int32 {
	if !ni.Valid {
		return nil
	}
	return &ni.Int32
}

// ptrToNullInt32 converts *int32 to sql.NullInt32
func ptrToNullInt32(i *int32) sql.NullInt32 {
	if i == nil {
		return sql.NullInt32{Valid: false}
	}
	return sql.NullInt32{Int32: *i, Valid: true}
}

// stringPtrToValue converts *string to string value (empty string if nil)
func stringPtrToValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
