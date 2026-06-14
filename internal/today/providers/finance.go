package providers

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/edwinupegui/arsenal/internal/finance"
	"github.com/edwinupegui/arsenal/internal/store"
	"github.com/edwinupegui/arsenal/internal/today"
)

// FinanceProvider contributes "This Month's Spending" and "Recent Transactions"
// sections to the Today view.
type FinanceProvider struct {
	queries *store.Queries
	db      *sql.DB
	now     func() time.Time
}

// FinanceProviderOption configures a FinanceProvider.
type FinanceProviderOption func(*FinanceProvider)

// WithFinanceClock replaces the default time.Now source. Used by tests to pin
// the wall-clock for date-sensitive comparisons.
func WithFinanceClock(now func() time.Time) FinanceProviderOption {
	return func(p *FinanceProvider) { p.now = now }
}

// NewFinanceProvider builds a FinanceProvider backed by db.
func NewFinanceProvider(db *sql.DB, opts ...FinanceProviderOption) *FinanceProvider {
	p := &FinanceProvider{queries: store.New(db), db: db, now: time.Now}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the provider identifier.
func (p *FinanceProvider) Name() string { return "finance" }

// Sections returns up to two sections: this-month-spending and recent-transactions.
// It uses the user's configured timezone for month boundaries and omits empty
// sections. Errors are returned so the registry can degrade gracefully.
func (p *FinanceProvider) Sections(ctx context.Context) ([]today.Section, error) {
	loc, err := today.UserLocation(ctx, p.db)
	if err != nil {
		return nil, err
	}
	now := p.now().In(loc)
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)
	monthStart := startOfMonth.Format("2006-01-02")
	monthEnd := endOfMonth.Format("2006-01-02")

	var sections []today.Section

	// This Month's Spending
	monthRows, err := p.queries.ListFinanceByMonth(ctx, store.ListFinanceByMonthParams{
		Date:   monthStart,
		Date_2: monthEnd,
	})
	if err != nil {
		return nil, fmt.Errorf("month spending query: %w", err)
	}
	topCats, err := p.queries.TopCategoriesByMonth(ctx, store.TopCategoriesByMonthParams{
		Date:   monthStart,
		Date_2: monthEnd,
		Limit:  3,
	})
	if err != nil {
		return nil, fmt.Errorf("top categories query: %w", err)
	}
	if len(monthRows) > 0 {
		var total float64
		var currency string
		for _, row := range monthRows {
			total += row.Amount
			currency = row.Currency
		}
		sections = append(sections, today.Section{
			Key:   "this-month-spending",
			Title: "This Month's Spending",
			Items: []today.Item{{
				Domain:   "finance",
				Title:    fmt.Sprintf("Total: %s", finance.FormatAmount(total, currency)),
				Subtitle: formatTopCategories(topCats, currency),
			}},
			IsEmpty: false,
		})
	}

	// Recent Transactions
	recentRows, err := p.queries.ListFinanceFiltered(ctx, store.FinanceListFilter{
		Limit: 100, // practical upper bound; today.Service.Build caps to 5
	})
	if err != nil {
		return nil, fmt.Errorf("recent transactions query: %w", err)
	}
	if len(recentRows) > 0 {
		recentItems := make([]today.Item, 0, len(recentRows))
		for _, row := range recentRows {
			recentItems = append(recentItems, mapFinanceItem(row))
		}
		sections = append(sections, today.Section{
			Key:     "recent-transactions",
			Title:   "Recent Transactions",
			Items:   recentItems,
			IsEmpty: false,
		})
	}

	return sections, nil
}

func mapFinanceItem(row store.ListedFinance) today.Item {
	return today.Item{
		Domain:   "finance",
		ID:       row.Finance.ID,
		Title:    fmt.Sprintf("%s (%s)", row.Finance.Account, row.Finance.Kind),
		Subtitle: finance.FormatAmount(row.Finance.Amount, row.Finance.Currency),
		Priority: "",
		Tags:     row.Tags,
		URL:      fmt.Sprintf("/finance/%d", row.Finance.ID),
	}
}

func formatTopCategories(rows []store.TopCategoriesByMonthRow, currency string) string {
	if len(rows) == 0 {
		return ""
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		total, err := finance.ToFloat64(row.Total)
		if err != nil {
			continue
		}
		name := "Uncategorized"
		if row.CategoryName != nil && *row.CategoryName != "" {
			name = *row.CategoryName
		}
		parts = append(parts, fmt.Sprintf("%s %s", name, finance.FormatAmount(total, currency)))
	}
	return strings.Join(parts, ", ")
}
