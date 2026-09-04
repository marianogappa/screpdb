package db

func ptrBool(v bool) *bool { return &v }

func ptrString(v string) *string { return &v }

func ptrFloat64(v float64) *float64 { return &v }
