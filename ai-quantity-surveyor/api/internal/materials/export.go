package materials

import (
	"context"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

func (service *Service) Export(ctx context.Context, id string) ([]byte, error) {
	comparison, found, err := service.repository.GetComparison(ctx, id)
	if err != nil {
		return nil, fail(503, "MaterialStoreUnavailable", "Comparison store unavailable.")
	}
	if !found {
		return nil, fail(404, "NotFound", "Saved comparison unavailable.")
	}
	budget, found, err := service.repository.GetBudget(ctx, comparison.BudgetID)
	if err != nil {
		return nil, fail(503, "MaterialStoreUnavailable", "Budget store unavailable.")
	}
	if !found {
		return nil, fail(409, "SourceRevisionUnavailable", "Comparison source budget unavailable.")
	}
	book := excelize.NewFile()
	defer book.Close()
	book.SetSheetName("Sheet1", "Requirements")
	for _, name := range []string{"Offers", "Sources", "Assumptions"} {
		if _, err := book.NewSheet(name); err != nil {
			return nil, err
		}
	}
	write := func(sheet string, row int, values ...any) error {
		for index, value := range values {
			cell, _ := excelize.CoordinatesToCellName(index+1, row)
			if err := book.SetCellValue(sheet, cell, value); err != nil {
				return err
			}
		}
		return nil
	}
	if err := write("Requirements", 1, "Requirement", "Rooms", "Trade", "Product / specification", "Source area m²", "Base amount", "Required amount", "Unit", "Pack size", "Pack unit", "Whole packs", "Purchased", "Surplus", "Coats", "Coverage m²/litre/coat", "Input origin", "Formula"); err != nil {
		return nil, err
	}
	if err := write("Offers", 1, "Observation", "Supplier", "Branch", "Currency", "Pack price", "Material subtotal", "Origin"); err != nil {
		return nil, err
	}
	if err := write("Sources", 1, "Observation", "Origin", "Observed at", "Availability", "Tax included", "Delivery"); err != nil {
		return nil, err
	}
	for index, item := range comparison.Basket.Items {
		requirement := item.Requirement
		if err := write("Requirements", index+2, item.RequirementID, strings.Join(requirement.RoomIDs, ", "), requirement.Trade, requirement.Recipe.Specification, requirement.SourceAreaM2, requirement.BaseAmount, requirement.RequiredAmount, requirement.Unit, requirement.Recipe.PackAmount, requirement.Recipe.PackUnit, requirement.PackCount, requirement.PurchasedAmount, requirement.Surplus, requirement.Recipe.Coats, requirement.Recipe.CoverageM2PerLitrePerCoat, requirement.Recipe.InputOrigin, requirement.FormulaVersion); err != nil {
			return nil, err
		}
		if err := write("Offers", index+2, item.Offer.ID, item.Offer.Supplier, item.Offer.Branch, item.Offer.Currency, item.Offer.Price, item.Subtotal, item.Offer.Origin); err != nil {
			return nil, err
		}
		delivery := any("Unknown")
		if item.Offer.Delivery != nil {
			delivery = *item.Offer.Delivery
		}
		tax := any("Unknown")
		if item.Offer.TaxIncluded != nil {
			tax = *item.Offer.TaxIncluded
		}
		if err := write("Sources", index+2, item.Offer.ID, item.Offer.Origin, item.Offer.ObservedAt.Format("2006-01-02T15:04:05Z07:00"), item.Offer.Availability, tax, delivery); err != nil {
			return nil, err
		}
	}
	total := any("Incomplete: unknown charges or stale source")
	if comparison.Basket.DeliveredTotal != nil {
		total = *comparison.Basket.DeliveredTotal
	}
	if err := write("Assumptions", 1, "Comparison", comparison.ID, "Version", comparison.Version); err != nil {
		return nil, err
	}
	if err := write("Assumptions", 2, "Material-only subtotal", comparison.Basket.MaterialSubtotal, comparison.Currency); err != nil {
		return nil, err
	}
	if err := write("Assumptions", 3, "Delivered total", total); err != nil {
		return nil, err
	}
	metadata := [][]any{
		{"Budget", budget.ID, "Version", budget.Version},
		{"Estimate", budget.EstimateID, "Analysis", budget.SourceAnalysisID},
		{"Confirmation revision", budget.SourceConfirmationRevision, "Drawing", budget.SourceFloorPlanID},
		{"Selected page", budget.SourcePageNumber, "Image digest", budget.SourceImageDigest},
		{"Formula", budget.FormulaVersion, "Material waste % (once)", budget.MaterialWastePercent},
		{"Location", comparison.Location.Country, comparison.Location.City, comparison.Location.Postcode},
	}
	if comparison.Conversion != nil {
		metadata = append(metadata, []any{"Manual conversion", comparison.Conversion.BaseCurrency, comparison.Conversion.QuoteCurrency, comparison.Conversion.Rate, comparison.Conversion.CapturedAt.Format("2006-01-02T15:04:05Z07:00")})
	}
	for index, values := range metadata {
		if err := write("Assumptions", index+4, values...); err != nil {
			return nil, err
		}
	}
	for index, warning := range comparison.Basket.Warnings {
		if err := write("Assumptions", index+4+len(metadata), fmt.Sprintf("Warning %d", index+1), warning); err != nil {
			return nil, err
		}
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
