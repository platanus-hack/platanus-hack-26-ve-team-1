package api

import "strconv"

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func atoi(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}
