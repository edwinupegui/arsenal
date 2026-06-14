package finance

import (
	"fmt"
)

// ToFloat64 converts a SQLite aggregate value (often returned as interface{})
// into a float64. SQLite's SUM() may return int64 or float64 depending on the
// column type and build, so this helper handles the common numeric kinds.
func ToFloat64(v interface{}) (float64, error) {
	if v == nil {
		return 0, nil
	}
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint8:
		return float64(n), nil
	case uint16:
		return float64(n), nil
	case uint32:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	case string:
		var f float64
		if _, err := fmt.Sscanf(n, "%f", &f); err != nil {
			return 0, fmt.Errorf("cannot parse %q as float64: %w", n, err)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// FormatAmount returns a human-readable amount with a currency symbol.
// It maps common ISO codes to symbols; unknown or empty currencies fall back
// to the code itself (or just the number when empty).
func FormatAmount(amount float64, currency string) string {
	if currency == "" {
		return fmt.Sprintf("%.2f", amount)
	}
	symbol := currencySymbol(currency)
	return fmt.Sprintf("%s%.2f", symbol, amount)
}

func currencySymbol(currency string) string {
	switch currency {
	case "USD", "ARS", "CAD", "AUD", "NZD", "HKD", "SGD":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "BRL":
		return "R$"
	case "MXN":
		return "$"
	case "JPY", "CNY":
		return "¥"
	default:
		return currency + " "
	}
}

