package disbursement

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Currency is an ISO 4217 currency supported by this application.
type Currency string

const (
	USD Currency = "USD"
	EUR Currency = "EUR"
)

// MinorUnits is an exact count of a currency's smallest supported unit.
type MinorUnits int64

// Money is an exact amount in one supported currency.
type Money struct {
	currency   Currency
	minorUnits MinorUnits
}

var ErrInvalidMoney = errors.New("invalid money")

// ParseMoney converts a canonical two-decimal USD or EUR amount into minor units.
func ParseMoney(amount string, currency Currency) (Money, error) {
	if currency != USD && currency != EUR {
		return Money{}, fmt.Errorf("%w: unsupported currency %q", ErrInvalidMoney, currency)
	}

	majorPart, fractionalPart, found := strings.Cut(amount, ".")
	if !found || majorPart == "" || len(fractionalPart) != 2 {
		return Money{}, fmt.Errorf("%w: amount must use exactly two fractional digits", ErrInvalidMoney)
	}
	if !containsOnlyDigits(majorPart) || !containsOnlyDigits(fractionalPart) {
		return Money{}, fmt.Errorf("%w: amount must contain only decimal digits", ErrInvalidMoney)
	}

	majorUnits, err := strconv.ParseInt(majorPart, 10, 64)
	if err != nil {
		return Money{}, fmt.Errorf("%w: whole units are out of range", ErrInvalidMoney)
	}
	fractionalUnits := int64(fractionalPart[0]-'0')*10 + int64(fractionalPart[1]-'0')
	if majorUnits > (math.MaxInt64-fractionalUnits)/100 {
		return Money{}, fmt.Errorf("%w: minor units are out of range", ErrInvalidMoney)
	}

	return Money{
		currency:   currency,
		minorUnits: MinorUnits(majorUnits*100 + fractionalUnits),
	}, nil
}

func containsOnlyDigits(value string) bool {
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}

	return true
}

func (m Money) Currency() Currency {
	return m.currency
}

func (m Money) MinorUnits() MinorUnits {
	return m.minorUnits
}

func (m Money) String() string {
	minorUnits := int64(m.minorUnits)
	return fmt.Sprintf("%d.%02d", minorUnits/100, minorUnits%100)
}
