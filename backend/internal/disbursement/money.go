package disbursement

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Currency is an ISO 4217 currency supported by this application.
type Currency string

const (
	// USD is United States dollar with two supported fractional digits.
	USD Currency = "USD"
	// EUR is euro with two supported fractional digits.
	EUR Currency = "EUR"
)

// MinorUnits is an exact count of a currency's smallest supported unit.
type MinorUnits int64

// Money is an exact amount in one supported currency.
type Money struct {
	currency   Currency
	minorUnits MinorUnits
}

// ErrInvalidMoney reports an unsupported currency or malformed decimal amount.
var ErrInvalidMoney = errors.New("invalid money")

const (
	fractionalDigitCount   = 2
	minorUnitsPerMajorUnit = int64(100)
)

// ParseMoney converts a canonical two-decimal USD or EUR amount into minor units.
func ParseMoney(amount string, currency Currency) (Money, error) {
	if err := validateCurrency(currency); err != nil {
		return Money{}, err
	}
	minorUnits, err := parseMinorUnits(amount)
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

func parseMinorUnits(amount string) (MinorUnits, error) {
	majorPart, fractionalPart, found := strings.Cut(amount, ".")
	if !found || majorPart == "" || len(fractionalPart) != fractionalDigitCount {
		return 0, fmt.Errorf("%w: amount must use exactly two fractional digits", ErrInvalidMoney)
	}
	if !containsOnlyDigits(majorPart) || !containsOnlyDigits(fractionalPart) {
		return 0, fmt.Errorf("%w: amount must contain only decimal digits", ErrInvalidMoney)
	}

	// Removing the decimal separator gives the exact minor-unit count: "1500.50" -> "150050".
	parsedMinorUnits, err := strconv.ParseInt(majorPart+fractionalPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: minor units are out of range", ErrInvalidMoney)
	}
	return MinorUnits(parsedMinorUnits), nil
}

// containsOnlyDigits rejects signs, separators, whitespace, and non-ASCII numerals.
func containsOnlyDigits(value string) bool {
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}

	return true
}

// Currency returns the amount's ISO 4217 currency.
func (m Money) Currency() Currency {
	return m.currency
}

// MinorUnits returns the exact smallest-unit count.
func (m Money) MinorUnits() MinorUnits {
	return m.minorUnits
}

// String returns the canonical two-decimal representation.
func (m Money) String() string {
	minorUnits := int64(m.minorUnits)
	return fmt.Sprintf(
		"%d.%02d",
		minorUnits/minorUnitsPerMajorUnit,
		minorUnits%minorUnitsPerMajorUnit,
	)
}
