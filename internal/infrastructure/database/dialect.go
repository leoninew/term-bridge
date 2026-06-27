package database

func Placeholder(driver string, index int) string {
	return "?"
}

func NowExpr(driver string) string {
	if driver == "mysql" {
		return "UTC_TIMESTAMP(6)"
	}
	return "strftime('%Y-%m-%dT%H:%M:%fZ', 'now')"
}
