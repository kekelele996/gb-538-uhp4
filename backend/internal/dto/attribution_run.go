package dto

import "time"

type CreateAttributionRunRequest struct {
	MeasurementIDs   []uint `json:"measurement_ids" binding:"required,min=1,dive,gt=0"`
	SourceProfileIDs []uint `json:"source_profile_ids" binding:"required,min=1,dive,gt=0"`
}

type ReviewAttributionRequest struct {
	Note    string `json:"note" binding:"required,min=3,max=1000"`
	Version uint   `json:"version" binding:"required,gte=1"`
}

type AttributionActionRequest struct {
	Version uint `json:"version" binding:"required,gte=1"`
}

type BandContribution struct {
	BandHz      int     `json:"band_hz"`
	PredictedDB float64 `json:"predicted_db"`
	EnergyShare float64 `json:"energy_share"`
}

type SourceContribution struct {
	SourceProfileID uint               `json:"source_profile_id"`
	SourceCode      string             `json:"source_code"`
	SourceName      string             `json:"source_name"`
	Coefficient     float64            `json:"coefficient"`
	ContributionPct float64            `json:"contribution_pct"`
	OverallDB       float64            `json:"overall_db"`
	Bands           []BandContribution `json:"bands"`
}

type AttributionEvidence struct {
	MatrixRows      int      `json:"matrix_rows"`
	MatrixColumns   int      `json:"matrix_columns"`
	Iterations      int      `json:"iterations"`
	Converged       bool     `json:"converged"`
	ConditionHint   float64  `json:"condition_hint"`
	UnreliableBands []string `json:"unreliable_bands"`
	Warnings        []string `json:"warnings"`
	Objective       float64  `json:"objective"`
	ElapsedMillis   int64    `json:"elapsed_millis"`
}

type AttributionRunResponse struct {
	ID               uint                 `json:"id"`
	RunCode          string               `json:"run_code"`
	MeasurementIDs   []uint               `json:"measurement_ids"`
	SourceProfileIDs []uint               `json:"source_profile_ids"`
	AlgorithmVersion string               `json:"algorithm_version"`
	InputHash        string               `json:"input_hash"`
	InputSnapshot    any                  `json:"input_snapshot"`
	NormalizedBands  any                  `json:"normalized_bands"`
	Contributions    []SourceContribution `json:"contributions"`
	Evidence         AttributionEvidence  `json:"evidence"`
	ResidualError    float64              `json:"residual_error"`
	AttributionState string               `json:"attribution_state"`
	Explanation      string               `json:"explanation"`
	StartedAt        time.Time            `json:"started_at"`
	FinishedAt       *time.Time           `json:"finished_at"`
	CreatedBy        uint                 `json:"created_by"`
	ReviewedBy       *uint                `json:"reviewed_by"`
	ReviewNote       string               `json:"review_note"`
	Version          uint                 `json:"version"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type AttributionComparisonSource struct {
	SourceProfileID uint    `json:"source_profile_id"`
	SourceCode      string  `json:"source_code"`
	SourceName      string  `json:"source_name"`
	InBase          bool    `json:"in_base"`
	InOther         bool    `json:"in_other"`
	BasePct         float64 `json:"base_pct"`
	OtherPct        float64 `json:"other_pct"`
	DeltaPct        float64 `json:"delta_pct"`
	BaseOverallDB   float64 `json:"base_overall_db"`
	OtherOverallDB  float64 `json:"other_overall_db"`
}

type AttributionComparisonBand struct {
	BandHz           int     `json:"band_hz"`
	BaseObservedDB   float64 `json:"base_observed_db"`
	OtherObservedDB  float64 `json:"other_observed_db"`
	ObservedDeltaDB  float64 `json:"observed_delta_db"`
	BasePredictedDB  float64 `json:"base_predicted_db"`
	OtherPredictedDB float64 `json:"other_predicted_db"`
	PredictedDeltaDB float64 `json:"predicted_delta_db"`
}

type AttributionComparisonResponse struct {
	BaseRunID              uint                          `json:"base_run_id"`
	OtherRunID             uint                          `json:"other_run_id"`
	BaseRunCode            string                        `json:"base_run_code"`
	OtherRunCode           string                        `json:"other_run_code"`
	BaseAlgorithmVersion   string                        `json:"base_algorithm_version"`
	OtherAlgorithmVersion  string                        `json:"other_algorithm_version"`
	BaseMeasurementIDs     []uint                        `json:"base_measurement_ids"`
	OtherMeasurementIDs    []uint                        `json:"other_measurement_ids"`
	BasePointIDs           []uint                        `json:"base_point_ids"`
	OtherPointIDs          []uint                        `json:"other_point_ids"`
	Comparable             bool                          `json:"comparable"`
	IncomparabilityReasons []string                      `json:"incomparability_reasons"`
	ResidualDelta          float64                       `json:"residual_delta"`
	TopSourceChanged       bool                          `json:"top_source_changed"`
	BaseTopSource          string                        `json:"base_top_source"`
	OtherTopSource         string                        `json:"other_top_source"`
	SourceDeltas           []AttributionComparisonSource `json:"source_deltas"`
	BandDeltas             []AttributionComparisonBand   `json:"band_deltas"`
	TopBandDeltas          []AttributionComparisonBand   `json:"top_band_deltas"`
	Explanation            string                        `json:"explanation"`
}
