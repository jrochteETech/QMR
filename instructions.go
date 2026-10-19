package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
)

type instructionData struct {
	Overview     [][]string `json:"overview"`
	Instructions [][]string `json:"instructions"`
	ScoringGuide [][]string `json:"scoringGuide"`
	SourceMatrix [][]string `json:"sourceMatrix"`
}

func loadInstructions(files fs.FS) (instructionData, error) {
	content, err := fs.ReadFile(files, "Web/instructions.json")
	if err != nil {
		return instructionData{}, fmt.Errorf("read instructions: %w", err)
	}
	var instructions instructionData
	if err := json.Unmarshal(content, &instructions); err != nil {
		return instructionData{}, fmt.Errorf("decode instructions: %w", err)
	}
	if len(instructions.Overview) == 0 || len(instructions.Instructions) == 0 || len(instructions.ScoringGuide) == 0 || len(instructions.SourceMatrix) == 0 {
		return instructionData{}, fmt.Errorf("instructions are incomplete")
	}
	return instructions, nil
}
