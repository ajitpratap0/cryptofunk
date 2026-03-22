package db

// PtrFloat64 returns a pointer to f. Useful for optional float64 struct fields.
func PtrFloat64(f float64) *float64 { return &f }
