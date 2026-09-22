package util

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

type StructuredTable struct {
	FileName string
	Sheet    string
	Headers  []string
	Rows     []StructuredTableRow
}

type StructuredTableRow struct {
	Number int
	Values []string
}

type structuredSourceRow struct {
	Number int
	Cells  []string
}

func ExtractStructuredTables(path string) ([]StructuredTable, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv":
		return extractStructuredCSV(path)
	case ".xlsx":
		return extractStructuredXLSX(path)
	default:
		return nil, fmt.Errorf("unsupported structured table type: %s", ext)
	}
}

func extractStructuredCSV(path string) ([]StructuredTable, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}

	table := buildStructuredTable(filepath.Base(path), "", records)
	if len(table.Headers) == 0 {
		return nil, nil
	}
	return []StructuredTable{table}, nil
}

func extractStructuredXLSX(path string) ([]StructuredTable, error) {
	workbook, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() { _ = workbook.Close() }()

	tables := make([]StructuredTable, 0)
	for _, sheet := range workbook.GetSheetList() {
		rows, err := workbook.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("read xlsx sheet %s: %w", sheet, err)
		}
		table := buildStructuredTable(filepath.Base(path), sheet, rows)
		if len(table.Headers) == 0 {
			continue
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func buildStructuredTable(fileName, sheet string, rows [][]string) StructuredTable {
	headers, sourceRows := prepareStructuredRows(rows)
	if len(headers) == 0 {
		return StructuredTable{}
	}

	tableRows := make([]StructuredTableRow, 0, len(sourceRows))
	for _, sourceRow := range sourceRows {
		values := make([]string, len(headers))
		for cellIndex := range headers {
			if cellIndex < len(sourceRow.Cells) {
				values[cellIndex] = strings.TrimSpace(sourceRow.Cells[cellIndex])
			}
		}
		tableRows = append(tableRows, StructuredTableRow{
			Number: sourceRow.Number,
			Values: values,
		})
	}

	return StructuredTable{
		FileName: fileName,
		Sheet:    sheet,
		Headers:  headers,
		Rows:     tableRows,
	}
}

func prepareStructuredRows(rows [][]string) ([]string, []structuredSourceRow) {
	nonEmptyRows := make([]structuredSourceRow, 0, len(rows))
	for index, row := range rows {
		if !rowHasContent(row) {
			continue
		}
		nonEmptyRows = append(nonEmptyRows, structuredSourceRow{
			Number: index + 1,
			Cells:  trimTrailingEmptyCells(row),
		})
	}
	if len(nonEmptyRows) == 0 {
		return nil, nil
	}

	headerIndex := 0
	bestCellCount := 0
	for index, row := range nonEmptyRows {
		cellCount := 0
		for _, cell := range row.Cells {
			if strings.TrimSpace(cell) != "" {
				cellCount++
			}
		}
		if cellCount > bestCellCount {
			bestCellCount = cellCount
			headerIndex = index
		}
	}

	headers := normalizeTableHeaders(nonEmptyRows[headerIndex].Cells)
	return headers, nonEmptyRows[headerIndex+1:]
}
