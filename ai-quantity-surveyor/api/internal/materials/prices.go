package materials

import (
	"context"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/domain"
)

type Offer struct {
	ID           string                `json:"id"`
	Supplier     string                `json:"supplier"`
	Branch       string                `json:"branch"`
	Country      string                `json:"country"`
	City         string                `json:"city"`
	Currency     string                `json:"currency"`
	Price        float64               `json:"price"`
	Recipe       domain.MaterialRecipe `json:"recipe"`
	Origin       string                `json:"origin"`
	ObservedAt   time.Time             `json:"observedAt"`
	Availability string                `json:"availability"`
	TaxIncluded  *bool                 `json:"taxIncluded"`
	Delivery     *float64              `json:"delivery"`
}

type Location struct {
	Country  string `json:"country"`
	City     string `json:"city"`
	Postcode string `json:"postcode,omitempty"`
}
type Selection struct {
	RequirementID string `json:"requirementId"`
	ObservationID string `json:"observationId"`
}
type Conversion struct {
	BaseCurrency  string    `json:"baseCurrency"`
	QuoteCurrency string    `json:"quoteCurrency"`
	Rate          float64   `json:"rate"`
	CapturedAt    time.Time `json:"capturedAt"`
}
type PreviewRequest struct {
	Location     Location      `json:"location"`
	Alternatives [][]Selection `json:"alternatives"`
	Conversion   *Conversion   `json:"conversion,omitempty"`
}
type ComparisonRequest struct {
	Location   Location    `json:"location"`
	Selections []Selection `json:"selections"`
	Conversion *Conversion `json:"conversion,omitempty"`
}
type PricedItem struct {
	RequirementID string                     `json:"requirementId"`
	Offer         Offer                      `json:"offer"`
	Requirement   domain.MaterialRequirement `json:"requirement"`
	Subtotal      float64                    `json:"subtotal"`
}
type Basket struct {
	Items            []PricedItem `json:"items"`
	MaterialSubtotal float64      `json:"materialSubtotal"`
	KnownDelivery    float64      `json:"knownDelivery"`
	DeliveredTotal   *float64     `json:"deliveredTotal"`
	Warnings         []string     `json:"warnings"`
}
type Preview struct {
	BudgetID     string    `json:"budgetId"`
	EvaluatedAt  time.Time `json:"evaluatedAt"`
	Currency     string    `json:"currency"`
	Alternatives []Basket  `json:"alternatives"`
}
type Comparison struct {
	ID         string      `json:"id"`
	BudgetID   string      `json:"budgetId"`
	ProjectID  string      `json:"projectId"`
	Version    int         `json:"version"`
	Location   Location    `json:"location"`
	Currency   string      `json:"currency"`
	Conversion *Conversion `json:"conversion,omitempty"`
	Basket     Basket      `json:"basket"`
	CreatedAt  time.Time   `json:"createdAt"`
}

func fixtureOffers() []Offer {
	created := time.Now().UTC()
	paint := domain.MaterialRecipe{Category: "paint", Specification: "Interior matt", Coats: 2, CoverageM2PerLitrePerCoat: 10, PackAmount: 5, PackUnit: "litre", InputOrigin: "manual"}
	tile := domain.MaterialRecipe{Category: "tile", Specification: "Floor tiles", PackAmount: 1.5, PackUnit: "m2", InputOrigin: "manual"}
	known := 1200.0
	taxIncluded := true
	return []Offer{
		{ID: "fixture-paint-a", Supplier: "Sample Supplier A (fictional)", Branch: "Sample Colombo branch", Country: "LK", City: "Colombo", Currency: "LKR", Price: 8500, Recipe: paint, Origin: "fixture", ObservedAt: created, Availability: "inStock", TaxIncluded: &taxIncluded, Delivery: &known},
		{ID: "fixture-paint-b", Supplier: "Sample Supplier B (fictional)", Branch: "Sample Colombo branch", Country: "LK", City: "Colombo", Currency: "LKR", Price: 7900, Recipe: paint, Origin: "fixture", ObservedAt: created, Availability: "inStock", TaxIncluded: &taxIncluded},
		{ID: "fixture-tile-a", Supplier: "Sample Supplier A (fictional)", Branch: "Sample Colombo branch", Country: "LK", City: "Colombo", Currency: "LKR", Price: 3200, Recipe: tile, Origin: "fixture", ObservedAt: created, Availability: "inStock", TaxIncluded: &taxIncluded, Delivery: &known},
	}
}

func (service *Service) Offers(ctx context.Context) ([]Offer, error) {
	return service.repository.ListOffers(ctx)
}

func (service *Service) Preview(ctx context.Context, budgetID string, input PreviewRequest) (Preview, error) {
	budget, found, err := service.repository.GetBudget(ctx, budgetID)
	if err != nil {
		return Preview{}, fail(503, "MaterialStoreUnavailable", "Budget store unavailable.")
	}
	if !found {
		return Preview{}, fail(404, "NotFound", "Material budget not found.")
	}
	if input.Location.Country != "LK" || input.Location.City != "Colombo" {
		return Preview{}, fail(400, "LocationUnsupported", "Fixture coverage is Colombo, Sri Lanka only.")
	}
	if len(input.Alternatives) == 0 || len(input.Alternatives) > 20 {
		return Preview{}, fail(400, "InvalidSelection", "Use between 1 and 20 alternatives.")
	}
	offers, err := service.repository.ListOffers(ctx)
	if err != nil {
		return Preview{}, fail(503, "MaterialStoreUnavailable", "Catalog unavailable.")
	}
	result := Preview{BudgetID: budget.ID, EvaluatedAt: time.Now().UTC(), Currency: "LKR", Alternatives: make([]Basket, 0)}
	count := 0
	for _, alternative := range input.Alternatives {
		count += len(alternative)
		if count > 256 {
			return Preview{}, fail(400, "InvalidSelection", "Too many offer selections.")
		}
		basket, err := priceBasket(budget, offers, alternative, input.Conversion, result.EvaluatedAt, false)
		if err != nil {
			return Preview{}, err
		}
		result.Alternatives = append(result.Alternatives, basket)
	}
	return result, nil
}

func priceBasket(budget Budget, observations []Offer, selections []Selection, conversion *Conversion, now time.Time, saving bool) (Basket, error) {
	if len(selections) == 0 || len(selections) > 256 {
		return Basket{}, fail(400, "InvalidSelection", "Select between 1 and 256 offers.")
	}
	requirements := make(map[string]domain.MaterialRequirement)
	for _, requirement := range budget.Requirements {
		requirements[requirement.ID] = requirement
	}
	offers := make(map[string]Offer)
	for _, offer := range observations {
		offers[offer.ID] = offer
	}
	seen := make(map[string]bool)
	delivery := make(map[string]*big.Rat)
	subtotal := new(big.Rat)
	basket := Basket{Items: make([]PricedItem, 0), Warnings: []string{"Fictional fixture prices; not live quotations or stock reservations."}}
	complete := true
	for _, selection := range selections {
		if seen[selection.RequirementID] {
			return Basket{}, fail(400, "InvalidSelection", "A requirement cannot be purchased twice in one basket.")
		}
		seen[selection.RequirementID] = true
		requirement, exists := requirements[selection.RequirementID]
		offer, available := offers[selection.ObservationID]
		if !exists || !available {
			return Basket{}, fail(404, "NotFound", "Selected requirement or observation unavailable.")
		}
		if offer.Country != "LK" || offer.City != "Colombo" {
			return Basket{}, fail(400, "IncomparableOffer", "Offer is outside the selected pilot location.")
		}
		if offer.Availability != "inStock" {
			return Basket{}, fail(409, "OfferUnavailable", "Selected offer is not available.")
		}
		if requirement.Recipe.Category != offer.Recipe.Category || requirement.Recipe.Specification != offer.Recipe.Specification || (requirement.Trade == domain.MaterialPaint && requirement.Recipe.Coats != offer.Recipe.Coats) {
			return Basket{}, fail(400, "IncomparableOffer", "Selected specification is not equivalent.")
		}
		recalculated, err := domain.CalculateMaterials([]domain.MaterialAllocation{{RoomID: "basket-allocation", Trade: requirement.Trade, AreaM2: requirement.SourceAreaM2, Recipe: offer.Recipe}}, budget.MaterialWastePercent)
		if err != nil {
			return Basket{}, fail(400, "InvalidMaterialRecipe", err.Error())
		}
		quantity := recalculated[0]
		price, err := moneyRat(offer.Price)
		if err != nil || offer.Price <= 0 {
			return Basket{}, fail(400, "InvalidPrice", "Offer price must be positive and finite.")
		}
		rate := big.NewRat(1, 1)
		if offer.Currency != "LKR" {
			if conversion == nil || conversion.BaseCurrency != offer.Currency || conversion.QuoteCurrency != "LKR" || conversion.CapturedAt.IsZero() || conversion.Rate <= 0 {
				return Basket{}, fail(400, "MissingConversion", "Review a captured conversion to LKR first.")
			}
			rate, err = moneyRat(conversion.Rate)
			if err != nil {
				return Basket{}, err
			}
			price.Mul(price, rate)
		}
		line := new(big.Rat).Mul(price, big.NewRat(quantity.PackCount, 1))
		subtotal.Add(subtotal, line)
		basket.Items = append(basket.Items, PricedItem{RequirementID: requirement.ID, Offer: offer, Requirement: quantity, Subtotal: roundMoney(line)})
		if now.Sub(offer.ObservedAt) > 24*time.Hour || offer.ObservedAt.After(now.Add(time.Minute)) {
			if saving {
				return Basket{}, fail(409, "OfferSelectionExpired", "Offer expired; review a fresh observation before saving.")
			}
			basket.Warnings = append(basket.Warnings, "Stale observation: "+offer.ID)
			complete = false
		}
		if offer.TaxIncluded == nil {
			complete = false
			basket.Warnings = append(basket.Warnings, "Tax status unknown: "+offer.ID)
		} else if !*offer.TaxIncluded {
			complete = false
			basket.Warnings = append(basket.Warnings, "Tax excluded from offer price: "+offer.ID)
		}
		if offer.Delivery == nil {
			complete = false
			basket.Warnings = append(basket.Warnings, "Delivery unknown: "+offer.ID)
		} else {
			charge, err := moneyRat(*offer.Delivery)
			if err != nil {
				return Basket{}, err
			}
			charge.Mul(charge, rate)
			key := offer.Supplier + "/" + offer.Branch
			if previous, found := delivery[key]; found && previous.Cmp(charge) != 0 {
				complete = false
				basket.Warnings = append(basket.Warnings, "Delivery rules differ: "+offer.Branch)
			} else {
				delivery[key] = charge
			}
		}
	}
	for _, requirement := range budget.Requirements {
		if requirement.RequiredAmount > 0 && !seen[requirement.ID] {
			return Basket{}, fail(400, "InvalidSelection", "Select an offer for every positive material requirement.")
		}
	}
	basket.MaterialSubtotal = roundMoney(subtotal)
	knownDelivery := new(big.Rat)
	for _, charge := range delivery {
		knownDelivery.Add(knownDelivery, charge)
	}
	basket.KnownDelivery = roundMoney(knownDelivery)
	if complete {
		amount := roundMoney(new(big.Rat).Add(subtotal, knownDelivery))
		basket.DeliveredTotal = &amount
	}
	if math.IsInf(basket.MaterialSubtotal, 0) || math.IsInf(basket.KnownDelivery, 0) || (basket.DeliveredTotal != nil && math.IsInf(*basket.DeliveredTotal, 0)) {
		return Basket{}, fail(400, "InvalidPrice", "Basket amount exceeds the supported numeric range.")
	}
	return basket, nil
}

func moneyRat(value float64) (*big.Rat, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return nil, fail(400, "InvalidPrice", "Price or conversion must be finite and nonnegative.")
	}
	result, ok := new(big.Rat).SetString(strconv.FormatFloat(value, 'f', -1, 64))
	if !ok {
		return nil, fail(400, "InvalidPrice", "Invalid decimal price.")
	}
	return result, nil
}
func roundMoney(value *big.Rat) float64 {
	scaled := new(big.Rat).Mul(value, big.NewRat(100, 1))
	whole, remainder := new(big.Int), new(big.Int)
	whole.QuoRem(scaled.Num(), scaled.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(scaled.Denom()) >= 0 {
		whole.Add(whole, big.NewInt(1))
	}
	result, _ := new(big.Rat).SetFrac(whole, big.NewInt(100)).Float64()
	return result
}
