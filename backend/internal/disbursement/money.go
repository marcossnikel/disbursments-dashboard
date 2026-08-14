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

const (
	decimalRadix           = 10
	fractionalDigitCount   = 2
	minorUnitsPerMajorUnit = int64(100)
)

// ParseMoney converts a canonical two-decimal USD or EUR amount into minor units.
func ParseMoney(amount string, currency Currency) (Money, error) {
	if err := validateCurrency(currency); err != nil {
		return Money{}, err
	}
	majorUnits, fractionalUnits, err := parseCanonicalAmount(amount)
	if err != nil {
		return Money{}, err
	}
	minorUnits, err := combineMinorUnits(majorUnits, fractionalUnits)
	if err != nil {
		return Money{}, err
	}

	return Money{
		currency:   currency,
		minorUnits: minorUnits,
	}, nil
}

func validateCurrency(currency Currency) error {
	if currency != USD && currency != EUR {
		return fmt.Errorf("%w: unsupported currency %q", ErrInvalidMoney, currency)
	}
	return nil
}

func parseCanonicalAmount(amount string) (int64, int64, error) {
	majorPart, fractionalPart, found := strings.Cut(amount, ".")
	if !found || majorPart == "" || len(fractionalPart) != fractionalDigitCount {
		return 0, 0, fmt.Errorf("%w: amount must use exactly two fractional digits", ErrInvalidMoney)
	}
	if !containsOnlyDigits(majorPart) || !containsOnlyDigits(fractionalPart) {
		return 0, 0, fmt.Errorf("%w: amount must contain only decimal digits", ErrInvalidMoney)
	}

	majorUnits, err := strconv.ParseInt(majorPart, decimalRadix, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: whole units are out of range", ErrInvalidMoney)
	}
	fractionalUnits, err := strconv.ParseInt(fractionalPart, decimalRadix, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: fractional units are out of range", ErrInvalidMoney)
	}
	return majorUnits, fractionalUnits, nil
}

func combineMinorUnits(majorUnits, fractionalUnits int64) (MinorUnits, error) {
	maximumSafeMajorUnits := (math.MaxInt64 - fractionalUnits) / minorUnitsPerMajorUnit
	if majorUnits > maximumSafeMajorUnits {
		return 0, fmt.Errorf("%w: minor units are out of range", ErrInvalidMoney)
	}
	return MinorUnits(majorUnits*minorUnitsPerMajorUnit + fractionalUnits), nil
}

// containsOnlyDigits rejects signs, separators, whitespace, and non-ASCII numerals.
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
	return fmt.Sprintf(
		"%d.%02d",
		minorUnits/minorUnitsPerMajorUnit,
		minorUnits%minorUnitsPerMajorUnit,
	)
}
