package estimate

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/example/ai-quantity-surveyor/api/internal/domain"
	"github.com/xuri/excelize/v2"
)

func (service *Service) Export(id string) ([]byte, error) {
	version, ok := service.GetByID(id)
	if !ok {
		return nil, fmt.Errorf("estimate %q not found", id)
	}
	workbook := excelize.NewFile()
	defer workbook.Close()
	workbook.SetSheetName("Sheet1", "Summary")
	rooms, err := workbook.NewSheet("Rooms")
	if err != nil {
		return nil, err
	}
	openings, err := workbook.NewSheet("Openings")
	if err != nil {
		return nil, err
	}
	workbook.SetActiveSheet(rooms)
	set := func(sheet, cell string, value any) { _ = workbook.SetCellValue(sheet, cell, value) }
	result := version.Result
	summary := [][]any{
		{"AI Quantity Surveyor Estimate"},
		{"Project ID", version.ProjectID},
		{"Estimate version", version.Version},
		{"Target currency", version.TargetCurrency},
		{"Exchange rate", version.ExchangeRate},
		{"Total plaster quantity (m2)", result.TotalPlasterQuantity},
		{"Total plaster cost (USD)", result.TotalPlasterCostUSD},
		{"Total paint quantity (m2)", result.TotalPaintQuantity},
		{"Total paint cost (USD)", result.TotalPaintCostUSD},
		{"Grand total (USD)", result.GrandTotalUSD},
		{"Grand total (target currency)", version.ConvertedGrandTotal},
		{"Total wall tiling quantity (m2)", result.TotalTilingQuantity},
		{"Total wall tiling cost (USD)", result.TotalTilingCostUSD},
		{"Total flooring quantity (m2)", result.TotalFlooringQuantity},
		{"Total flooring cost (USD)", result.TotalFlooringCostUSD},
		{"Total ceiling quantity (m2)", result.TotalCeilingQuantity},
		{"Total ceiling cost (USD)", result.TotalCeilingCostUSD},
		{"Total skirting quantity (m)", result.TotalSkirtingQuantity},
		{"Total skirting cost (USD)", result.TotalSkirtingCostUSD},
	}
	for row, values := range summary {
		for column, value := range values {
			set("Summary", cell(column, row+1), value)
		}
	}

	roomHeaders := []string{"Room", "Floor area (m2)", "Gross wall area (m2)", "Door deduction (m2)", "Window deduction (m2)", "Net plaster area (m2)", "Paint area (m2)", "Plaster cost (USD)", "Paint cost (USD)", "Perimeter (m)", "Wall tile area (m2)", "Flooring area (m2)", "Ceiling area (m2)", "Skirting length (m)", "Tiling cost (USD)", "Flooring cost (USD)", "Ceiling cost (USD)", "Skirting cost (USD)", "Room total (USD)"}
	for column, header := range roomHeaders {
		set("Rooms", cell(column, 1), header)
	}
	for row, room := range result.Rooms {
		values := []any{room.RoomName, room.FloorArea, room.GrossWallArea, room.DoorDeduction, room.WindowDeduction, room.NetPlasterArea, room.PaintArea, room.PlasterCostUSD, room.PaintCostUSD, room.Perimeter, room.TileArea, room.FlooringArea, room.CeilingArea, room.SkirtingLength, room.TilingCostUSD, room.FlooringCostUSD, room.CeilingCostUSD, room.SkirtingCostUSD, room.TotalCostUSD}
		for column, value := range values {
			set("Rooms", cell(column, row+2), value)
		}
	}
	openingHeaders := []string{"Room", "Opening type", "Width", "Height", "Area deduction (m2)"}
	for column, header := range openingHeaders {
		set("Openings", cell(column, 1), header)
	}
	_ = openings
	if _, err := workbook.NewSheet("Assumptions"); err != nil {
		return nil, err
	}
	assumptions := [][]any{{"Assumption", "Value"}, {"Exchange rate source", "Entered manually"}, {"Exchange rate entered at", version.ExchangeRateFetchedAt}}
	if rates := version.Rates; rates != nil {
		assumptions = append(assumptions,
			[]any{"Plaster rate (USD/m2)", rates.PlasterUSDPerSquareMeter}, []any{"Paint rate (USD/m2)", rates.PaintUSDPerSquareMeter},
			[]any{"Wall tiling rate (USD/m2)", rates.TilingUSDPerSquareMeter}, []any{"Flooring rate (USD/m2)", rates.FlooringUSDPerSquareMeter},
			[]any{"Ceiling rate (USD/m2)", rates.CeilingUSDPerSquareMeter}, []any{"Skirting rate (USD/m)", rates.SkirtingUSDPerMeter},
			[]any{"Waste allowance (%)", rates.WastePercent})
	}
	for _, room := range result.Rooms {
		if finish, ok := version.Finishes[room.RoomID]; ok {
			assumptions = append(assumptions, []any{"Finishes: " + room.RoomName, describeFinish(finish)})
		}
	}
	for row, values := range assumptions {
		for column, value := range values {
			set("Assumptions", cell(column, row+1), value)
		}
	}
	var output bytes.Buffer
	if err := workbook.Write(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func describeFinish(finish domain.RoomFinish) string {
	var parts []string
	for _, item := range []struct {
		excluded bool
		name     string
	}{{finish.ExcludePlaster, "plaster"}, {finish.ExcludePaint, "paint"}, {finish.ExcludeFlooring, "flooring"}, {finish.ExcludeCeiling, "ceiling"}, {finish.ExcludeSkirting, "skirting"}} {
		if item.excluded {
			parts = append(parts, "no "+item.name)
		}
	}
	if finish.TilePercent > 0 {
		parts = append(parts, fmt.Sprintf("%g%% wall tiling", finish.TilePercent))
	}
	return strings.Join(parts, ", ")
}

func cell(column, row int) string {
	name, _ := excelize.CoordinatesToCellName(column+1, row)
	return name
}
