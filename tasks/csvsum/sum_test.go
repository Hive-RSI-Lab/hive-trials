//go:build acceptance

package csvsum

import (
	"strings"
	"testing"
)

type sumTestCase struct {
	name    string
	csvData string
	column  string
	wantSum float64
	wantErr bool
}

func TestSum(t *testing.T) {
	cases := []sumTestCase{
		{
			name:    "normal",
			csvData: "name,age,score\nAlice,30,10.5\nBob,25,20\nCarol,22,5.5",
			column:  "score",
			wantSum: 36.0,
			wantErr: false,
		},
		{
			name:    "missing column",
			csvData: "name,age\nAlice,30\nBob,25",
			column:  "score",
			wantSum: 0,
			wantErr: true,
		},
		{
			name:    "bad number",
			csvData: "name,score\nAlice,ten\nBob,20",
			column:  "score",
			wantSum: 0,
			wantErr: true,
		},
		{
			name:    "empty file",
			csvData: "",
			column:  "score",
			wantSum: 0,
			wantErr: false,
		},
		{
			name:    "empty cell counts as zero",
			csvData: "name,score\nAlice,10\nBob,\nCarol,20",
			column:  "score",
			wantSum: 30.0,
			wantErr: false,
		},
		{
			name:    "header only",
			csvData: "name,score\n",
			column:  "score",
			wantSum: 0,
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := strings.NewReader(tc.csvData)
			got, err := Sum(r, tc.column)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected error status: %v, want error: %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.wantSum {
				t.Fatalf("expected sum %v, got %v", tc.wantSum, got)
			}
		})
	}
}
