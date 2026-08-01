package officetext

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ExtractXLSX converts every sheet of a workbook to a markdown table, in
// workbook sheet order. Formulas are rendered as their last calculated value
// (excelize's GetRows resolves cached results, matching what a user would
// see with the workbook open), not as the formula text.
func ExtractXLSX(data []byte) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("not a valid XLSX: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return "", fmt.Errorf("not a valid XLSX: no sheets found")
	}

	var sb strings.Builder
	for _, sheet := range sheets {
		rows, err := f.GetRows(sheet)
		if err != nil || len(rows) == 0 {
			continue
		}

		sb.WriteString("## ")
		sb.WriteString(sheet)
		sb.WriteString("\n\n")
		writeSheetTable(rows, &sb)
		sb.WriteByte('\n')
	}

	return strings.TrimSpace(sb.String()), nil
}

func writeSheetTable(rows [][]string, sb *strings.Builder) {
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	if maxCols == 0 {
		return
	}

	writeRow := func(row []string) {
		sb.WriteByte('|')
		for i := 0; i < maxCols; i++ {
			sb.WriteByte(' ')
			if i < len(row) {
				cell := strings.ReplaceAll(row[i], "|", "\\|")
				cell = strings.ReplaceAll(cell, "\n", " ")
				sb.WriteString(cell)
			}
			sb.WriteString(" |")
		}
		sb.WriteByte('\n')
	}

	writeRow(rows[0])
	sb.WriteByte('|')
	for i := 0; i < maxCols; i++ {
		sb.WriteString(" --- |")
	}
	sb.WriteByte('\n')
	for _, row := range rows[1:] {
		writeRow(row)
	}
}
