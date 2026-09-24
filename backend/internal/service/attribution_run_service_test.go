package service

import (
	"testing"
	"time"

	"industrial-noise-source-attribution/backend/internal/algorithm"
	"industrial-noise-source-attribution/backend/internal/constants"
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

func comparisonBands(predictedDB map[int]float64) []dto.BandContribution {
	bands := make([]dto.BandContribution, 0, len(constants.OctaveBands))
	for _, band := range constants.OctaveBands {
		bands = append(bands, dto.BandContribution{BandHz: band, PredictedDB: predictedDB[band], EnergyShare: 0})
	}
	return bands
}

func comparisonNormalized(levels map[int]float64) []algorithm.Spectrum {
	spectrum := make(algorithm.Spectrum, len(constants.OctaveBands))
	for _, band := range constants.OctaveBands {
		spectrum[algorithm.BandKey(band)] = levels[band]
	}
	return []algorithm.Spectrum{spectrum}
}

func flatLevels(level float64) map[int]float64 {
	levels := make(map[int]float64, len(constants.OctaveBands))
	for _, band := range constants.OctaveBands {
		levels[band] = level
	}
	return levels
}

func sourceEntry(id uint, code, name string, pct, overall float64, predictedByBand map[int]float64) dto.SourceContribution {
	return dto.SourceContribution{
		SourceProfileID: id, SourceCode: code, SourceName: name,
		ContributionPct: pct, OverallDB: overall, Bands: comparisonBands(predictedByBand),
	}
}

func TestBuildAttributionComparisonSourceAndBandDeltas(t *testing.T) {
	base := comparisonInput{
		RunCode: "AR-BASE", AlgorithmVersion: "octave-nnls-v1.0.0",
		MeasurementIDs: []uint{10}, PointIDs: []uint{1},
		Normalized: comparisonNormalized(flatLevels(80)),
		Contributions: []dto.SourceContribution{
			sourceEntry(1, "PUMP-01", "泵", 70, 84, flatLevels(78)),
			sourceEntry(2, "FAN-02", "风机", 30, 76, flatLevels(70)),
		},
		ResidualError: 0.1,
	}
	otherLevels := flatLevels(80)
	otherLevels[250] = 90
	otherLevels[8000] = 65
	other := comparisonInput{
		RunCode: "AR-OLD", AlgorithmVersion: "octave-nnls-v1.0.0",
		MeasurementIDs: []uint{11}, PointIDs: []uint{1},
		Normalized: comparisonNormalized(otherLevels),
		Contributions: []dto.SourceContribution{
			sourceEntry(2, "FAN-02", "风机", 60, 79, flatLevels(72)),
			sourceEntry(1, "PUMP-01", "泵", 40, 80, flatLevels(75)),
		},
		ResidualError: 0.2,
	}
	result := buildAttributionComparison(1, 2, base, other)
	if !result.Comparable {
		t.Fatalf("same algorithm and point set must be comparable, reasons=%v", result.IncomparabilityReasons)
	}
	if result.ResidualDelta != 0.1 {
		t.Fatalf("residual delta = %v, want 0.1", result.ResidualDelta)
	}
	if len(result.SourceDeltas) != 2 {
		t.Fatalf("source deltas = %d, want 2", len(result.SourceDeltas))
	}
	pump := result.SourceDeltas[0]
	if pump.SourceProfileID != 1 || pump.DeltaPct != -30 {
		t.Fatalf("equal |delta| should tie-break by source id: %+v", result.SourceDeltas)
	}
	if result.SourceDeltas[1].DeltaPct != 30 {
		t.Fatalf("fan delta should be +30: %+v", result.SourceDeltas[1])
	}
	for _, entry := range result.SourceDeltas {
		if !entry.InBase || !entry.InOther {
			t.Fatalf("both sources should be present in both runs: %+v", entry)
		}
		if got := entry.OtherPct - entry.BasePct; got != entry.DeltaPct {
			t.Fatalf("delta %v does not equal other-base %v", entry.DeltaPct, got)
		}
	}
	if len(result.TopBandDeltas) != 3 {
		t.Fatalf("top bands = %d, want 3", len(result.TopBandDeltas))
	}
	if result.TopBandDeltas[0].BandHz != 8000 {
		t.Fatalf("largest band change must be 8000 Hz (-15), got %d", result.TopBandDeltas[0].BandHz)
	}
	if result.TopBandDeltas[1].BandHz != 250 {
		t.Fatalf("second largest band change must be 250 Hz (+10), got %d", result.TopBandDeltas[1].BandHz)
	}
	if result.TopBandDeltas[0].ObservedDeltaDB != -15 {
		t.Fatalf("8000 Hz observed delta = %v, want -15", result.TopBandDeltas[0].ObservedDeltaDB)
	}
	if result.TopBandDeltas[1].ObservedDeltaDB != 10 {
		t.Fatalf("250 Hz observed delta = %v, want 10", result.TopBandDeltas[1].ObservedDeltaDB)
	}
	if len(result.BandDeltas) != len(constants.OctaveBands) {
		t.Fatalf("full band table = %d, want %d", len(result.BandDeltas), len(constants.OctaveBands))
	}
	if !result.TopSourceChanged {
		t.Fatalf("top source changed between runs, flag must be true")
	}
}

func TestBuildAttributionComparisonFlagsAlgorithmAndPointMismatch(t *testing.T) {
	base := comparisonInput{
		RunCode: "AR-BASE", AlgorithmVersion: "octave-nnls-v2.0.0",
		MeasurementIDs: []uint{10}, PointIDs: []uint{1},
		Normalized: comparisonNormalized(flatLevels(80)),
		Contributions: []dto.SourceContribution{
			sourceEntry(1, "PUMP-01", "泵", 90, 84, flatLevels(80)),
		},
	}
	other := comparisonInput{
		RunCode: "AR-OLD", AlgorithmVersion: "octave-nnls-v1.0.0",
		MeasurementIDs: []uint{20}, PointIDs: []uint{2},
		Normalized: comparisonNormalized(flatLevels(79)),
		Contributions: []dto.SourceContribution{
			sourceEntry(1, "PUMP-01", "泵", 55, 80, flatLevels(78)),
			sourceEntry(3, "VALVE-03", "阀", 45, 76, flatLevels(70)),
		},
	}
	result := buildAttributionComparison(1, 2, base, other)
	if result.Comparable {
		t.Fatalf("different algorithm and point sets must not be comparable")
	}
	if len(result.IncomparabilityReasons) != 2 {
		t.Fatalf("incomparability reasons = %v, want 2", result.IncomparabilityReasons)
	}
	if len(result.SourceDeltas) != 2 {
		t.Fatalf("itemized deltas must still be returned, got %d", len(result.SourceDeltas))
	}
	byID := map[uint]dto.AttributionComparisonSource{}
	for _, entry := range result.SourceDeltas {
		byID[entry.SourceProfileID] = entry
	}
	valve := byID[3]
	if valve.InBase || !valve.InOther || valve.DeltaPct != 0 {
		t.Fatalf("history-only source should be flagged without delta: %+v", valve)
	}
	pump := byID[1]
	if pump.DeltaPct != -35 {
		t.Fatalf("pump delta = %v, want -35 (history 55 minus current 90)", pump.DeltaPct)
	}
}
