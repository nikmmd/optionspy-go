package main

import (
	"testing"
)

func TestReadInput(t *testing.T) {
	rows := ReadInput("./symbols_test.txt")
	expected := []string{"^VIX", "^SPX", " "}
	if len(rows) != len(expected) {
		t.Error("Read incorrent")
	}
}
