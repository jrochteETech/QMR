package main

import "testing"

func TestParseInstructionWorkbook(t *testing.T) {
	instructions, err := parseInstructionWorkbook(instructionWorkbook)
	if err != nil {
		t.Fatalf("parseInstructionWorkbook() error = %v", err)
	}

	tests := []struct {
		name string
		rows [][]string
	}{
		{name: "overview", rows: instructions.Overview},
		{name: "instructions", rows: instructions.Instructions},
		{name: "scoring guide", rows: instructions.ScoringGuide},
		{name: "source matrix", rows: instructions.SourceMatrix},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if len(test.rows) < 2 {
				t.Fatalf("expected header and content rows, got %d rows", len(test.rows))
			}
			hasMultipleColumns := false
			for _, row := range test.rows {
				if len(row) >= 2 {
					hasMultipleColumns = true
					break
				}
			}
			if !hasMultipleColumns {
				t.Fatal("expected at least one multi-column row")
			}
		})
	}
}
