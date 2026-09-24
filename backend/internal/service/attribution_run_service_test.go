package service

import (
	"strings"
	"testing"
	"time"

	"industrial-noise-source-attribution/backend/internal/dto"
	"industrial-noise-source-attribution/backend/internal/model"
)

func TestAttributionResponseKeepsEqualIDArrays(t *testing.T) {
	run := model.AttributionRun{
		ID: 1, RunCode: "AR-TEST", MeasurementIDsJSON: "[1,2,3]", SourceProfileIDsJSON: "[1,2,3]",
		AlgorithmVersion: "test", InputHash: "hash", InputSnapshotJSON: "{}", NormalizedBandsJSON: "[]",
		ContributionsJSON: "[]", EvidenceJSON: "{}", AttributionState: "completed", StartedAt: time.Now(),
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	response, err := attributionResponse(run)
	if err != nil {
		t.Fatalf("attributionResponse returned error: %v", err)
	}
	if len(response.MeasurementIDs) != 3 || len(response.SourceProfileIDs) != 3 {
		t.Fatalf("equal JSON arrays overwrote each other: measurements=%v sources=%v", response.MeasurementIDs, response.SourceProfileIDs)
	}
}

func comparisonRun(id uint, code, algorithm string, pointIDs []uint, pointCodes []string, sources []dto.SourceContribution, residual float64) dto.AttributionRunResponse {
	measurements := make([]map[string]any, 0, len(pointIDs))
	for index, pointID := range pointIDs {
		measurements = append(measurements, map[string]any{
			"id": pointID, "checksum": "checksum",
			"point": map[string]any{"id": pointID, "point_code": pointCodes[index]},
		})
	}
	return dto.AttributionRunResponse{
		ID: id, RunCode: code, AlgorithmVersion: algorithm,
		MeasurementIDs: pointIDs, SourceProfileIDs: []uint{10, 11},
		InputSnapshot: map[string]any{"algorithm_version": algorithm, "measurements": measurements, "sources": []any{}},
		Contributions: sources, ResidualError: residual,
	}
}

func singleSourceContribution(id uint, code, name string, pct float64, bands map[int]float64) dto.SourceContribution {
	items := make([]dto.BandContribution, 0, len(bands))
	for band, level := range bands {
		items = append(items, dto.BandContribution{BandHz: band, PredictedDB: level, EnergyShare: pct})
	}
	return dto.SourceContribution{SourceProfileID: id, SourceCode: code, SourceName: name, ContributionPct: pct, Bands: items}
}

func TestBuildAttributionComparisonComparable(t *testing.T) {
	baseBands := map[int]float64{63: 60, 125: 60, 250: 60, 500: 60, 1000: 60, 2000: 60, 4000: 60, 8000: 60}
	otherBands := map[int]float64{63: 61, 125: 64, 250: 66, 500: 62, 1000: 60, 2000: 60, 4000: 60, 8000: 60}
	base := comparisonRun(1, "AR-BASE", "octave-nnls-v1.0.0", []uint{7, 8}, []string{"MP-A", "MP-B"},
		[]dto.SourceContribution{singleSourceContribution(10, "SRC-A", "Source A", 70, baseBands)}, 0.12)
	other := comparisonRun(2, "AR-OTHER", "octave-nnls-v1.0.0", []uint{8, 7}, []string{"MP-B", "MP-A"},
		[]dto.SourceContribution{singleSourceContribution(10, "SRC-A", "Source A", 64.5, otherBands)}, 0.18)

	result := buildAttributionComparison(base, other)
	if !result.Comparable || len(result.ComparabilityWarnings) != 0 {
		t.Fatalf("expected comparable runs, got warnings %v", result.ComparabilityWarnings)
	}
	if result.BaseRun.RunCode != "AR-BASE" || result.OtherRun.RunCode != "AR-OTHER" {
		t.Fatalf("unexpected run summaries: %+v / %+v", result.BaseRun, result.OtherRun)
	}
	if result.ResidualDelta != 0.06 {
		t.Fatalf("residual delta = %v, want 0.06", result.ResidualDelta)
	}
	if len(result.SourceDeltas) != 1 {
		t.Fatalf("source deltas = %d, want 1", len(result.SourceDeltas))
	}
	if delta := result.SourceDeltas[0]; delta.DeltaPct != -5.5 || !delta.InBase || !delta.InOther {
		t.Fatalf("unexpected source delta: %+v", delta)
	}
	if len(result.TopBandDeltas) != 3 {
		t.Fatalf("top band deltas = %d, want 3", len(result.TopBandDeltas))
	}
	wantBands := []int{250, 125, 500}
	for index, band := range wantBands {
		if result.TopBandDeltas[index].BandHz != band {
			t.Fatalf("top band %d = %d Hz, want %d Hz", index, result.TopBandDeltas[index].BandHz, band)
		}
	}
	if first := result.TopBandDeltas[0]; first.DeltaDB != 6 || first.AbsDeltaDB != 6 {
		t.Fatalf("largest band delta = %+v, want +6 dB", first)
	}
}

func TestBuildAttributionComparisonFlagsAlgorithmAndPoints(t *testing.T) {
	bands := map[int]float64{63: 60, 125: 60, 250: 60, 500: 60, 1000: 60, 2000: 60, 4000: 60, 8000: 60}
	base := comparisonRun(1, "AR-BASE", "octave-nnls-v1.0.0", []uint{7}, []string{"MP-A"},
		[]dto.SourceContribution{singleSourceContribution(10, "SRC-A", "Source A", 70, bands)}, 0.1)
	other := comparisonRun(2, "AR-OTHER", "octave-nnls-v1.1.0", []uint{9}, []string{"MP-C"},
		[]dto.SourceContribution{singleSourceContribution(10, "SRC-A", "Source A", 55, bands)}, 0.2)

	result := buildAttributionComparison(base, other)
	if result.Comparable || len(result.ComparabilityWarnings) != 2 {
		t.Fatalf("expected two comparability warnings, got comparable=%v warnings=%v", result.Comparable, result.ComparabilityWarnings)
	}
	joined := ""
	for _, warning := range result.ComparabilityWarnings {
		joined += warning + "\n"
	}
	if !strings.Contains(joined, "算法版本") || !strings.Contains(joined, "监测点集合") {
		t.Fatalf("warnings do not explain algorithm/point differences: %s", joined)
	}
	// Item-level deltas stay available even when runs are not directly comparable.
	if len(result.SourceDeltas) != 1 || result.SourceDeltas[0].DeltaPct != -15 {
		t.Fatalf("item-level source delta missing despite warning: %+v", result.SourceDeltas)
	}
	if len(result.TopBandDeltas) != 3 {
		t.Fatalf("top band deltas = %d, want 3", len(result.TopBandDeltas))
	}
}

func TestBuildAttributionComparisonHandlesSourceSetChanges(t *testing.T) {
	bands := map[int]float64{63: 60, 125: 60, 250: 60, 500: 60, 1000: 60, 2000: 60, 4000: 60, 8000: 60}
	base := comparisonRun(1, "AR-BASE", "octave-nnls-v1.0.0", []uint{7}, []string{"MP-A"},
		[]dto.SourceContribution{
			singleSourceContribution(10, "SRC-A", "Source A", 60, bands),
			singleSourceContribution(11, "SRC-B", "Source B", 40, bands),
		}, 0.1)
	other := comparisonRun(2, "AR-OTHER", "octave-nnls-v1.0.0", []uint{7}, []string{"MP-A"},
		[]dto.SourceContribution{
			singleSourceContribution(10, "SRC-A", "Source A", 75, bands),
			singleSourceContribution(12, "SRC-C", "Source C", 25, bands),
		}, 0.1)

	result := buildAttributionComparison(base, other)
	byID := make(map[uint]dto.AttributionSourceDelta, len(result.SourceDeltas))
	for _, delta := range result.SourceDeltas {
		byID[delta.SourceProfileID] = delta
	}
	a, ok := byID[10]
	if !ok || a.DeltaPct != 15 || !a.InBase || !a.InOther {
		t.Fatalf("shared source delta wrong: %+v ok=%v", a, ok)
	}
	b, ok := byID[11]
	if !ok || b.InOther || b.DeltaPct != -40 {
		t.Fatalf("base-only source delta wrong: %+v ok=%v", b, ok)
	}
	c, ok := byID[12]
	if !ok || c.InBase || c.DeltaPct != 25 {
		t.Fatalf("other-only source delta wrong: %+v ok=%v", c, ok)
	}
	wantOrder := []uint{11, 12, 10}
	for index, id := range wantOrder {
		if result.SourceDeltas[index].SourceProfileID != id {
			t.Fatalf("deltas should be ordered by absolute change, got %+v", result.SourceDeltas)
		}
	}
}
