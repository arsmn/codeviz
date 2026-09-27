package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const sample = "../../internal/adapters/goparser/testdata/sample"

func TestMinScoreGate(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"-o", "-", "-format", "json", sample}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "Score ") {
		t.Errorf("report has no score line:\n%s", errOut.String())
	}
	var scene struct {
		Summary struct {
			Score struct{ Total float64 } `json:"score"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &scene); err != nil {
		t.Fatal(err)
	}
	total := scene.Summary.Score.Total

	for _, tc := range []struct {
		threshold float64
		want      error
	}{
		{total - 1, nil},
		{total + 0.5, errBelowMinScore},
	} {
		out.Reset()
		err := run([]string{"-o", "-", "-format", "json", "-q", "-min-score", fmt.Sprint(tc.threshold), sample}, &out, &errOut)
		if !errors.Is(err, tc.want) {
			t.Errorf("-min-score %.1f with score %.1f: err = %v, want %v", tc.threshold, total, err, tc.want)
		}
		if out.Len() == 0 {
			t.Errorf("-min-score %.1f: output should be written either way", tc.threshold)
		}
	}

	err := run([]string{"-q", "-min-score", "101", sample}, &out, &errOut)
	if err == nil || errors.Is(err, errBelowMinScore) {
		t.Errorf("out-of-range threshold should be a usage error, got %v", err)
	}
}
