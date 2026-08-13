package disbursement_test

import (
	"testing"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestParseMoneyPreservesAnExactUSDDecimal(t *testing.T) {
	t.Parallel()

	money, err := disbursement.ParseMoney("1500.50", disbursement.USD)
	if err != nil {
		t.Fatalf("ParseMoney() error = %v", err)
	}

	if got, want := money.MinorUnits(), disbursement.MinorUnits(150050); got != want {
		t.Errorf("MinorUnits() = %d, want %d", got, want)
	}
	if got, want := money.Currency(), disbursement.USD; got != want {
		t.Errorf("Currency() = %q, want %q", got, want)
	}
	if got, want := money.String(), "1500.50"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestParseMoneyRejectsValuesOutsideTheSupportedContract(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		amount   string
		currency disbursement.Currency
	}{
		"missing fractional digits": {amount: "1", currency: disbursement.USD},
		"one fractional digit":      {amount: "1.0", currency: disbursement.USD},
		"three fractional digits":   {amount: "1.000", currency: disbursement.USD},
		"negative amount":           {amount: "-1.00", currency: disbursement.USD},
		"explicit positive sign":    {amount: "+1.00", currency: disbursement.USD},
		"decimal comma":             {amount: "1,00", currency: disbursement.USD},
		"non numeric":               {amount: "amount", currency: disbursement.USD},
		"unsupported currency":      {amount: "1.00", currency: "BRL"},
		"integer overflow":          {amount: "92233720368547758.08", currency: disbursement.EUR},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := disbursement.ParseMoney(testCase.amount, testCase.currency); err == nil {
				t.Fatal("ParseMoney() error = nil, want an error")
			}
		})
	}
}
