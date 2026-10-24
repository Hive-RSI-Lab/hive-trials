package csvsum

import (
    "strings"
    "testing"
)

func TestSum_Normal(t *testing.T) {
    csvData := "name,age,score\nAlice,30,10.5\nBob,25,20\nCarol,22,5.5"
    r := strings.NewReader(csvData)
    s, err := Sum(r, "score")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    expected := 36.0 // 10.5+20+5.5
    if s != expected {
        t.Fatalf("expected %v, got %v", expected, s)
    }
}

func TestSum_MissingColumn(t *testing.T) {
    csvData := "name,age\nAlice,30\nBob,25"
    r := strings.NewReader(csvData)
    _, err := Sum(r, "score")
    if err == nil {
        t.Fatalf("expected error for missing column")
    }
}

func TestSum_BadNumber(t *testing.T) {
    csvData := "name,score\nAlice,ten\nBob,20"
    r := strings.NewReader(csvData)
    _, err := Sum(r, "score")
    if err == nil {
        t.Fatalf("expected error for non‑numeric value")
    }
}

func TestSum_EmptyFile(t *testing.T) {
    r := strings.NewReader("")
    s, err := Sum(r, "score")
    if err != nil {
        t.Fatalf("unexpected error on empty file: %v", err)
    }
    if s != 0 {
        t.Fatalf("expected sum 0 for empty file, got %v", s)
    }
}

