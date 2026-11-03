package auth

import (
	"database/sql"
	"strconv"
)

type storeTx = sql.Tx

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func parseInt(s string) (int, error) { return strconv.Atoi(s) }
