package estimate

import (
	"bytes"
	"fmt"
	"strconv"

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
	set("Summary", "A1", "AI Quantity Surveyor Estimate")
	set("Summary", "A2", "Project ID")
	set("Summary", "B2", version.ProjectID)
	set("Summary", "A3", "Estimate version")
	set("Summary", "B3", version.Version)
	set("Summary", "A4", "Target currency")
	set("Summary", "B4", version.TargetCurrency)
	set("Summary", "A5", "Exchange rate")
	set("Summary", "B5", version.ExchangeRate)
	set("Summary", "A6", "Total plaster quantity (m2)")
	set("Summary", "B6", version.Result.TotalPlasterQuantity)
	set("Summary", "A7", "Total plaster cost (USD)")
	set("Summary", "B7", version.Result.TotalPlasterCostUSD)
	set("Summary", "A8", "Total paint quantity (m2)")
	set("Summary", "B8", version.Result.TotalPaintQuantity)
	set("Summary", "A9", "Total paint cost (USD)")
	set("Summary", "B9", version.Result.TotalPaintCostUSD)
	set("Summary", "A10", "Grand total (USD)")
	set("Summary", "B10", version.Result.GrandTotalUSD)
	set("Summary", "A11", "Grand total (target currency)")
	set("Summary", "B11", version.ConvertedGrandTotal)

	roomHeaders := []string{"Room", "Floor area (m2)", "Gross wall area (m2)", "Door deduction (m2)", "Window deduction (m2)", "Net plaster area (m2)", "Paint area (m2)", "Plaster cost (USD)", "Paint cost (USD)"}
	for column, header := range roomHeaders {
		set("Rooms", cell(column, 1), header)
	}
	for row, room := range version.Result.Rooms {
		values := []any{room.RoomName, room.FloorArea, room.GrossWallArea, room.DoorDeduction, room.WindowDeduction, room.NetPlasterArea, room.PaintArea, room.PlasterCostUSD, room.PaintCostUSD}
		for column, value := range values {
			set("Rooms", cell(column, row+2), value)
		}
	}
	openingHeaders := []string{"Room", "Opening type", "Width", "Height", "Area deduction (m2)"}
	for column, header := range openingHeaders {
		set("Openings", cell(column, 1), header)
	}
	_ = openings
	var output bytes.Buffer
	if err := workbook.Write(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func cell(column, row int) string { return string(rune('A'+column)) + strconv.Itoa(row) }
