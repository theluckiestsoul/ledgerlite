// Package money works in integer minor units (paise, cents); floats never touch a balance.
package money

import (
	"errors"
	"fmt"
)

type Currency string

const (
	INR Currency = "INR"
	USD Currency = "USD"
	EUR Currency = "EUR"
)

var Supported = []Currency{INR, USD, EUR}

// MaxAmount keeps values exact for JavaScript clients (2^53 - 1).
const MaxAmount int64 = 1<<53 - 1

var ErrOverflow = errors.New("money: amount overflow")

func ParseCurrency(s string) (Currency, bool) {
	for _, c := range Supported {
		if string(c) == s {
			return c, true
		}
	}
	return "", false
}

func ValidAmount(v int64) bool {
	return v > 0 && v <= MaxAmount
}

func Add(a, b int64) (int64, error) {
	if b > MaxAmount-a {
		return 0, ErrOverflow
	}
	return a + b, nil
}

func Format(amount int64, c Currency) string {
	sign := ""
	if amount < 0 {
		sign, amount = "-", -amount
	}
	return fmt.Sprintf("%s%d.%02d %s", sign, amount/100, amount%100, c)
}
