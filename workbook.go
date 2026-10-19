package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

type instructionData struct {
	Overview     [][]string `json:"overview"`
	Instructions [][]string `json:"instructions"`
	ScoringGuide [][]string `json:"scoringGuide"`
	SourceMatrix [][]string `json:"sourceMatrix"`
}

type workbookXML struct {
	Sheets []workbookSheet `xml:"sheets>sheet"`
}

type workbookSheet struct {
	Name string `xml:"name,attr"`
	ID   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

type relationshipsXML struct {
	Relationships []relationshipXML `xml:"Relationship"`
}

type relationshipXML struct {
	ID     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
}

type sharedStringsXML struct {
	Items []sharedStringXML `xml:"si"`
}

type sharedStringXML struct {
	Text string          `xml:"t"`
	Runs []sharedTextRun `xml:"r"`
}

type sharedTextRun struct {
	Text string `xml:"t"`
}

type worksheetXML struct {
	Rows []worksheetRow `xml:"sheetData>row"`
}

type worksheetRow struct {
	Cells []worksheetCell `xml:"c"`
}

type worksheetCell struct {
	Reference string          `xml:"r,attr"`
	Type      string          `xml:"t,attr"`
	Value     string          `xml:"v"`
	Inline    inlineStringXML `xml:"is"`
}

type inlineStringXML struct {
	Text string          `xml:"t"`
	Runs []sharedTextRun `xml:"r"`
}

func parseInstructionWorkbook(content []byte) (instructionData, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return instructionData{}, fmt.Errorf("open instruction workbook: %w", err)
	}
	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		files[path.Clean(file.Name)] = file
	}

	var workbook workbookXML
	if err := decodeWorkbookXML(files, "xl/workbook.xml", &workbook); err != nil {
		return instructionData{}, err
	}
	var relationships relationshipsXML
	if err := decodeWorkbookXML(files, "xl/_rels/workbook.xml.rels", &relationships); err != nil {
		return instructionData{}, err
	}
	targets := make(map[string]string, len(relationships.Relationships))
	for _, relationship := range relationships.Relationships {
		target := strings.TrimPrefix(relationship.Target, "/")
		if !strings.HasPrefix(target, "xl/") {
			target = path.Join("xl", target)
		}
		targets[relationship.ID] = target
	}

	sharedStrings := []string{}
	if _, ok := files["xl/sharedStrings.xml"]; ok {
		var shared sharedStringsXML
		if err := decodeWorkbookXML(files, "xl/sharedStrings.xml", &shared); err != nil {
			return instructionData{}, err
		}
		for _, item := range shared.Items {
			var value strings.Builder
			value.WriteString(item.Text)
			for _, run := range item.Runs {
				value.WriteString(run.Text)
			}
			sharedStrings = append(sharedStrings, value.String())
		}
	}

	sheets := make(map[string][][]string)
	for _, sheet := range workbook.Sheets {
		if sheet.Name != "Overview" && sheet.Name != "Instructions" && sheet.Name != "Scoring Guide" && sheet.Name != "Source Matrix" {
			continue
		}
		var worksheet worksheetXML
		if err := decodeWorkbookXML(files, targets[sheet.ID], &worksheet); err != nil {
			return instructionData{}, err
		}
		sheets[sheet.Name] = worksheetRows(worksheet.Rows, sharedStrings)
	}
	return instructionData{
		Overview:     sheets["Overview"],
		Instructions: sheets["Instructions"],
		ScoringGuide: sheets["Scoring Guide"],
		SourceMatrix: sheets["Source Matrix"],
	}, nil
}

func decodeWorkbookXML(files map[string]*zip.File, name string, target any) error {
	file, ok := files[path.Clean(name)]
	if !ok {
		return fmt.Errorf("instruction workbook is missing %s", name)
	}
	stream, err := file.Open()
	if err != nil {
		return fmt.Errorf("open %s: %w", name, err)
	}
	defer stream.Close()
	if err := xml.NewDecoder(stream).Decode(target); err != nil && err != io.EOF {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	return nil
}

func worksheetRows(rows []worksheetRow, sharedStrings []string) [][]string {
	result := make([][]string, 0, len(rows))
	for _, row := range rows {
		values := []string{}
		for _, cell := range row.Cells {
			column := cellColumn(cell.Reference)
			for len(values) <= column {
				values = append(values, "")
			}
			values[column] = workbookCellValue(cell, sharedStrings)
		}
		for len(values) > 0 && strings.TrimSpace(values[len(values)-1]) == "" {
			values = values[:len(values)-1]
		}
		if len(values) > 0 {
			result = append(result, values)
		}
	}
	return result
}

func workbookCellValue(cell worksheetCell, sharedStrings []string) string {
	switch cell.Type {
	case "s":
		index, err := strconv.Atoi(cell.Value)
		if err == nil && index >= 0 && index < len(sharedStrings) {
			return sharedStrings[index]
		}
	case "inlineStr":
		var value strings.Builder
		value.WriteString(cell.Inline.Text)
		for _, run := range cell.Inline.Runs {
			value.WriteString(run.Text)
		}
		return value.String()
	default:
		return cell.Value
	}
	return ""
}

func cellColumn(reference string) int {
	column := 0
	for _, character := range reference {
		if character < 'A' || character > 'Z' {
			break
		}
		column = column*26 + int(character-'A'+1)
	}
	if column == 0 {
		return 0
	}
	return column - 1
}
