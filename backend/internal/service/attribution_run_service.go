package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"industrial-noise-source-attribution/backend/internal/algorithm"
	"industrial-noise-source-attribution/backend/internal/constants"
	"industrial-noise-source-attribution/backend/internal/dto"
	"industrial-noise-source-attribution/backend/internal/model"
	"industrial-noise-source-attribution/backend/internal/repository"
	"industrial-noise-source-attribution/backend/internal/util"
)

type AttributionRunService struct {
	repository            *repository.AttributionRunRepository
	measurementRepository *repository.NoiseMeasurementRepository
	sourceRepository      *repository.SourceProfileRepository
}

func NewAttributionRunService(runRepo *repository.AttributionRunRepository, measurementRepo *repository.NoiseMeasurementRepository, sourceRepo *repository.SourceProfileRepository) *AttributionRunService {
	return &AttributionRunService{repository: runRepo, measurementRepository: measurementRepo, sourceRepository: sourceRepo}
}

func (s *AttributionRunService) List(ctx context.Context) ([]dto.AttributionRunResponse, error) {
	runs, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.AttributionRunResponse, 0, len(runs))
	for _, run := range runs {
		response, err := attributionResponse(run)
		if err != nil {
			return nil, err
		}
		result = append(result, response)
	}
	return result, nil
}

func (s *AttributionRunService) Get(ctx context.Context, id uint) (dto.AttributionRunResponse, error) {
	run, err := s.repository.Get(ctx, id)
	if err != nil {
		return dto.AttributionRunResponse{}, mapRepositoryError(err, "归因运行不存在", "归因运行读取冲突")
	}
	return attributionResponse(run)
}

func (s *AttributionRunService) Create(ctx context.Context, request dto.CreateAttributionRunRequest, actor model.Actor, idempotencyKey string) (dto.AttributionRunResponse, bool, error) {
	measurementIDs, err := algorithm.SortedUniqueIDs(request.MeasurementIDs)
	if err != nil {
		return dto.AttributionRunResponse{}, false, util.Validation("测量 ID 无效", err)
	}
	sourceIDs, err := algorithm.SortedUniqueIDs(request.SourceProfileIDs)
	if err != nil {
		return dto.AttributionRunResponse{}, false, util.Validation("声源谱 ID 无效", err)
	}
	measurements, err := s.measurementRepository.GetManyReady(ctx, measurementIDs)
	if err != nil {
		return dto.AttributionRunResponse{}, false, util.Validation("所有测量必须存在并处于 ready 状态", err)
	}
	sources, err := s.sourceRepository.GetManyActive(ctx, sourceIDs)
	if err != nil {
		return dto.AttributionRunResponse{}, false, util.Validation("所有声源谱必须存在并处于 active 状态", err)
	}
	sort.Slice(measurements, func(i, j int) bool { return measurements[i].ID < measurements[j].ID })
	sort.Slice(sources, func(i, j int) bool { return sources[i].ID < sources[j].ID })
	measurementInputs, err := buildMeasurementInputs(measurements)
	if err != nil {
		return dto.AttributionRunResponse{}, false, err
	}
	sourceInputs, err := buildSourceInputs(sources)
	if err != nil {
		return dto.AttributionRunResponse{}, false, err
	}
	snapshot := struct {
		AlgorithmVersion string                       `json:"algorithm_version"`
		Measurements     []algorithm.MeasurementInput `json:"measurements"`
		Sources          []algorithm.SourceInput      `json:"sources"`
	}{constants.AlgorithmVersion, measurementInputs, sourceInputs}
	inputHash, snapshotJSON, err := algorithm.CanonicalHash(snapshot)
	if err != nil {
		return dto.AttributionRunResponse{}, false, err
	}
	if existing, findErr := s.repository.FindByInput(ctx, inputHash, constants.AlgorithmVersion); findErr == nil {
		response, responseErr := attributionResponse(existing)
		return response, true, responseErr
	} else if !repository.IsNotFound(findErr) && !strings.Contains(findErr.Error(), gorm.ErrRecordNotFound.Error()) {
		return dto.AttributionRunResponse{}, false, findErr
	}
	result, err := algorithm.FitAttribution(measurementInputs, sourceInputs)
	if err != nil {
		return dto.AttributionRunResponse{}, false, util.Validation("归因算法无法处理当前冻结输入", err)
	}
	finished := time.Now().UTC()
	run := model.AttributionRun{
		RunCode:            "AR-" + strings.ToUpper(inputHash[:10]),
		MeasurementIDsJSON: mustJSON(measurementIDs), SourceProfileIDsJSON: mustJSON(sourceIDs),
		AlgorithmVersion: constants.AlgorithmVersion, InputHash: inputHash,
		InputSnapshotJSON: string(snapshotJSON), NormalizedBandsJSON: mustJSON(result.NormalizedBands),
		ContributionsJSON: mustJSON(result.Contributions), EvidenceJSON: mustJSON(result.Evidence),
		ResidualError: result.ResidualError, AttributionState: string(constants.AttributionCompleted),
		Explanation: result.Explanation, StartedAt: finished.Add(-time.Duration(result.Evidence.ElapsedMillis) * time.Millisecond),
		FinishedAt: &finished, CreatedBy: actor.ID, Version: 1, CreatedAt: finished, UpdatedAt: finished,
	}
	audit := newAudit(actor, "attribution_run.completed", "AttributionRun", 0, map[string]any{"state": constants.AttributionQueued}, run, map[string]any{
		"input_hash": inputHash, "algorithm_version": constants.AlgorithmVersion,
		"idempotency_key": idempotencyKey, "measurement_checksums": measurementChecksums(measurementInputs),
		"matrix_rows": result.Evidence.MatrixRows, "matrix_columns": result.Evidence.MatrixColumns,
		"iterations": result.Evidence.Iterations, "elapsed_millis": result.Evidence.ElapsedMillis,
	})
	if err := s.repository.CreateCalculated(ctx, &run, audit); err != nil {
		if existing, findErr := s.repository.FindByInput(ctx, inputHash, constants.AlgorithmVersion); findErr == nil {
			response, responseErr := attributionResponse(existing)
			return response, true, responseErr
		}
		return dto.AttributionRunResponse{}, false, mapRepositoryError(err, "归因运行不存在", "相同冻结输入正在或已经计算")
	}
	response, err := s.Get(ctx, run.ID)
	return response, false, err
}

func (s *AttributionRunService) Review(ctx context.Context, id uint, request dto.ReviewAttributionRequest, actor model.Actor) (dto.AttributionRunResponse, error) {
	return s.transition(ctx, id, request.Version, constants.AttributionReviewed, request.Note, actor)
}

func (s *AttributionRunService) Confirm(ctx context.Context, id uint, request dto.AttributionActionRequest, actor model.Actor) (dto.AttributionRunResponse, error) {
	run, err := s.repository.Get(ctx, id)
	if err != nil {
		return dto.AttributionRunResponse{}, mapRepositoryError(err, "归因运行不存在", "归因运行读取冲突")
	}
	if run.CreatedBy == actor.ID {
		return dto.AttributionRunResponse{}, util.Forbidden("归因运行发起人不得确认自己的结果")
	}
	return s.transitionLoaded(ctx, run, request.Version, constants.AttributionConfirmed, "", actor)
}

func (s *AttributionRunService) Void(ctx context.Context, id uint, request dto.AttributionActionRequest, actor model.Actor) (dto.AttributionRunResponse, error) {
	return s.transition(ctx, id, request.Version, constants.AttributionVoided, "", actor)
}

func (s *AttributionRunService) Compare(ctx context.Context, baseID, otherID uint) (dto.AttributionComparisonResponse, error) {
	if baseID == 0 || otherID == 0 {
		return dto.AttributionComparisonResponse{}, util.Validation("归因运行 ID 无效", fmt.Errorf("comparison run ids must be positive"))
	}
	if baseID == otherID {
		return dto.AttributionComparisonResponse{}, util.Validation("当前运行与历史运行不能是同一条记录", fmt.Errorf("base run %d equals other run", baseID))
	}
	baseRun, err := s.repository.Get(ctx, baseID)
	if err != nil {
		return dto.AttributionComparisonResponse{}, mapRepositoryError(err, "当前归因运行不存在", "归因运行读取冲突")
	}
	otherRun, err := s.repository.Get(ctx, otherID)
	if err != nil {
		return dto.AttributionComparisonResponse{}, mapRepositoryError(err, "历史归因运行不存在", "归因运行读取冲突")
	}
	base, err := attributionResponse(baseRun)
	if err != nil {
		return dto.AttributionComparisonResponse{}, err
	}
	other, err := attributionResponse(otherRun)
	if err != nil {
		return dto.AttributionComparisonResponse{}, err
	}
	return buildAttributionComparison(base, other), nil
}

func (s *AttributionRunService) transition(ctx context.Context, id, version uint, to constants.AttributionState, note string, actor model.Actor) (dto.AttributionRunResponse, error) {
	run, err := s.repository.Get(ctx, id)
	if err != nil {
		return dto.AttributionRunResponse{}, mapRepositoryError(err, "归因运行不存在", "归因运行读取冲突")
	}
	return s.transitionLoaded(ctx, run, version, to, note, actor)
}

func (s *AttributionRunService) transitionLoaded(ctx context.Context, run model.AttributionRun, version uint, to constants.AttributionState, note string, actor model.Actor) (dto.AttributionRunResponse, error) {
	from := constants.AttributionState(run.AttributionState)
	if !constants.CanTransitionAttribution(from, to) {
		return dto.AttributionRunResponse{}, util.Conflict(fmt.Sprintf("不允许从 %s 迁移到 %s", from, to), nil)
	}
	after := run
	after.AttributionState, after.Version = string(to), version+1
	var reviewer *uint
	if to == constants.AttributionReviewed || to == constants.AttributionConfirmed {
		reviewer = &actor.ID
		after.ReviewedBy = reviewer
	}
	if note != "" {
		after.ReviewNote = note
	}
	audit := newAudit(actor, "attribution_run.state_changed", "AttributionRun", run.ID, run, after, map[string]any{"from": from, "to": to, "expected_version": version})
	if err := s.repository.Transition(ctx, run.ID, version, string(from), string(to), reviewer, note, audit); err != nil {
		return dto.AttributionRunResponse{}, mapRepositoryError(err, "归因运行不存在", "归因运行状态或版本已变化")
	}
	return s.Get(ctx, run.ID)
}

func buildMeasurementInputs(measurements []model.NoiseMeasurement) ([]algorithm.MeasurementInput, error) {
	result := make([]algorithm.MeasurementInput, 0, len(measurements))
	for _, measurement := range measurements {
		bands, err := decodeSpectrum(measurement.OctaveBandsJSON)
		if err != nil {
			return nil, err
		}
		background, err := decodeSpectrum(measurement.MonitoringPoint.BackgroundProfileJSON)
		if err != nil {
			return nil, err
		}
		result = append(result, algorithm.MeasurementInput{
			ID: measurement.ID, Checksum: measurement.SourceChecksum, Bands: bands,
			Point: algorithm.PointInput{ID: measurement.MonitoringPoint.ID, PointCode: measurement.MonitoringPoint.PointCode,
				XM: measurement.MonitoringPoint.XM, YM: measurement.MonitoringPoint.YM,
				HeightM: measurement.MonitoringPoint.HeightM, Background: background},
		})
	}
	return result, nil
}

func buildSourceInputs(sources []model.SourceProfile) ([]algorithm.SourceInput, error) {
	result := make([]algorithm.SourceInput, 0, len(sources))
	for _, source := range sources {
		power, err := decodeSpectrum(source.OctavePowerJSON)
		if err != nil {
			return nil, err
		}
		directivity, err := decodeSpectrum(source.DirectivityJSON)
		if err != nil {
			return nil, err
		}
		result = append(result, algorithm.SourceInput{
			ID: source.ID, SourceCode: source.SourceCode, Name: source.Name,
			XM: source.XM, YM: source.YM, HeightM: source.HeightM,
			ReferenceDistanceM: source.ReferenceDistanceM, Power: power, Directivity: directivity,
			OperatingFactor: source.OperatingFactor, Version: source.Version,
		})
	}
	return result, nil
}

func attributionResponse(run model.AttributionRun) (dto.AttributionRunResponse, error) {
	var measurementIDs, sourceIDs []uint
	var contributions []dto.SourceContribution
	var evidence dto.AttributionEvidence
	var snapshot, normalized any
	fields := []struct {
		raw    string
		target any
	}{
		{run.MeasurementIDsJSON, &measurementIDs}, {run.SourceProfileIDsJSON, &sourceIDs},
		{run.ContributionsJSON, &contributions}, {run.EvidenceJSON, &evidence},
		{run.InputSnapshotJSON, &snapshot}, {run.NormalizedBandsJSON, &normalized},
	}
	for _, field := range fields {
		raw, target := field.raw, field.target
		if err := json.Unmarshal([]byte(raw), target); err != nil {
			return dto.AttributionRunResponse{}, fmt.Errorf("decode stored attribution evidence: %w", err)
		}
	}
	return dto.AttributionRunResponse{
		ID: run.ID, RunCode: run.RunCode, MeasurementIDs: measurementIDs, SourceProfileIDs: sourceIDs,
		AlgorithmVersion: run.AlgorithmVersion, InputHash: run.InputHash,
		InputSnapshot: snapshot, NormalizedBands: normalized, Contributions: contributions,
		Evidence: evidence, ResidualError: run.ResidualError, AttributionState: run.AttributionState,
		Explanation: run.Explanation, StartedAt: run.StartedAt, FinishedAt: run.FinishedAt,
		CreatedBy: run.CreatedBy, ReviewedBy: run.ReviewedBy, ReviewNote: run.ReviewNote,
		Version: run.Version, CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt,
	}, nil
}

type frozenSnapshot struct {
	AlgorithmVersion string `json:"algorithm_version"`
	Measurements     []struct {
		ID    uint `json:"id"`
		Point struct {
			ID        uint   `json:"id"`
			PointCode string `json:"point_code"`
		} `json:"point"`
	} `json:"measurements"`
}

type frozenSnapshotPoint struct {
	ID   uint
	Code string
}

// decodeFrozenPoints extracts the monitoring point set frozen inside an
// attribution run's canonical input snapshot. The snapshot is immutable
// evidence, so the comparison uses it rather than current entity state.
func decodeFrozenPoints(raw any) ([]frozenSnapshotPoint, error) {
	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal frozen snapshot: %w", err)
	}
	var snapshot frozenSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return nil, fmt.Errorf("decode frozen snapshot: %w", err)
	}
	seen := make(map[uint]bool, len(snapshot.Measurements))
	points := make([]frozenSnapshotPoint, 0, len(snapshot.Measurements))
	for _, measurement := range snapshot.Measurements {
		if measurement.Point.ID == 0 || seen[measurement.Point.ID] {
			continue
		}
		seen[measurement.Point.ID] = true
		points = append(points, frozenSnapshotPoint{ID: measurement.Point.ID, Code: measurement.Point.PointCode})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].ID < points[j].ID })
	return points, nil
}

func comparisonSummary(run dto.AttributionRunResponse) (dto.AttributionComparisonRun, []frozenSnapshotPoint, error) {
	points, err := decodeFrozenPoints(run.InputSnapshot)
	if err != nil {
		return dto.AttributionComparisonRun{}, nil, err
	}
	pointIDs := make([]uint, 0, len(points))
	pointCodes := make([]string, 0, len(points))
	for _, point := range points {
		pointIDs = append(pointIDs, point.ID)
		pointCodes = append(pointCodes, point.Code)
	}
	return dto.AttributionComparisonRun{
		RunID: run.ID, RunCode: run.RunCode, AlgorithmVersion: run.AlgorithmVersion,
		MeasurementCount: len(run.MeasurementIDs), SourceCount: len(run.SourceProfileIDs),
		MonitoringPointIDs: pointIDs, MonitoringPoints: pointCodes, FinishedAt: run.FinishedAt,
	}, points, nil
}

func buildAttributionComparison(base, other dto.AttributionRunResponse) dto.AttributionComparisonResponse {
	baseSummary, basePoints, err := comparisonSummary(base)
	otherSummary, otherPoints, otherErr := comparisonSummary(other)
	if err != nil || otherErr != nil {
		return dto.AttributionComparisonResponse{
			BaseRunID: base.ID, OtherRunID: other.ID, BaseRun: baseSummary, OtherRun: otherSummary,
			SourceDeltas: []dto.AttributionSourceDelta{}, TopBandDeltas: []dto.AttributionBandDelta{},
			Explanation: "无法解析冻结输入快照，逐项差异不可用；请检查历史记录完整性。",
		}
	}

	warnings := make([]string, 0, 2)
	if base.AlgorithmVersion != other.AlgorithmVersion {
		warnings = append(warnings, fmt.Sprintf(
			"两次运行算法版本不同（当前 %s，历史 %s），拟合口径不一致，贡献百分点不能直接比较。",
			base.AlgorithmVersion, other.AlgorithmVersion,
		))
	}
	basePointSet := make(map[uint]bool, len(basePoints))
	for _, point := range basePoints {
		basePointSet[point.ID] = true
	}
	otherPointSet := make(map[uint]bool, len(otherPoints))
	for _, point := range otherPoints {
		otherPointSet[point.ID] = true
	}
	if !sameUintSet(basePointSet, otherPointSet) {
		warnings = append(warnings, fmt.Sprintf(
			"两次运行冻结的监测点集合不同（当前 %s，历史 %s），观测对象不一致，差异只能用于判断变化来自哪次冻结输入。",
			strings.Join(baseSummary.MonitoringPoints, ", "), strings.Join(otherSummary.MonitoringPoints, ", "),
		))
	}

	baseTop, otherTop := "", ""
	if len(base.Contributions) > 0 {
		baseTop = base.Contributions[0].SourceCode
	}
	if len(other.Contributions) > 0 {
		otherTop = other.Contributions[0].SourceCode
	}

	response := dto.AttributionComparisonResponse{
		BaseRunID: base.ID, OtherRunID: other.ID, BaseRun: baseSummary, OtherRun: otherSummary,
		Comparable:            len(warnings) == 0,
		ComparabilityWarnings: warnings,
		ResidualDelta:         roundFloat(other.ResidualError-base.ResidualError, 6),
		TopSourceChanged:      baseTop != otherTop, BaseTopSource: baseTop, OtherTopSource: otherTop,
		SourceDeltas:  buildSourceDeltas(base, other),
		TopBandDeltas: buildTopBandDeltas(base, other, 3),
		Explanation:   "比较仅描述两个冻结输入与算法版本的离线结果差异，不覆盖任何历史记录；负值表示历史运行低于当前运行。",
	}
	return response
}

func sameUintSet(left, right map[uint]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for value := range left {
		if !right[value] {
			return false
		}
	}
	return true
}

func buildSourceDeltas(base, other dto.AttributionRunResponse) []dto.AttributionSourceDelta {
	index := make(map[uint]int)
	deltas := make([]dto.AttributionSourceDelta, 0, len(base.Contributions)+len(other.Contributions))
	for _, source := range base.Contributions {
		index[source.SourceProfileID] = len(deltas)
		deltas = append(deltas, dto.AttributionSourceDelta{
			SourceProfileID: source.SourceProfileID, SourceCode: source.SourceCode, SourceName: source.SourceName,
			BaseContribution: source.ContributionPct, InBase: true,
		})
	}
	for _, source := range other.Contributions {
		if position, ok := index[source.SourceProfileID]; ok {
			deltas[position].OtherContribution = source.ContributionPct
			deltas[position].InOther = true
			deltas[position].DeltaPct = roundFloat(source.ContributionPct-deltas[position].BaseContribution, 3)
			continue
		}
		index[source.SourceProfileID] = len(deltas)
		deltas = append(deltas, dto.AttributionSourceDelta{
			SourceProfileID: source.SourceProfileID, SourceCode: source.SourceCode, SourceName: source.SourceName,
			OtherContribution: source.ContributionPct, InOther: true,
			DeltaPct: roundFloat(source.ContributionPct, 3),
		})
	}
	// Sources that only exist in the current run carry the full base share as a negative move.
	for position := range deltas {
		if deltas[position].InBase && !deltas[position].InOther {
			deltas[position].DeltaPct = roundFloat(-deltas[position].BaseContribution, 3)
		}
	}
	sort.SliceStable(deltas, func(i, j int) bool {
		if math.Abs(deltas[i].DeltaPct) != math.Abs(deltas[j].DeltaPct) {
			return math.Abs(deltas[i].DeltaPct) > math.Abs(deltas[j].DeltaPct)
		}
		return deltas[i].SourceCode < deltas[j].SourceCode
	})
	return deltas
}

// aggregateBandEnergy sums every candidate source's predicted relative energy
// for one octave band, then converts the total back to a dB level so the two
// frozen runs share an absolute reference.
func aggregateBandLevel(run dto.AttributionRunResponse, band int) (float64, bool) {
	total := 0.0
	present := false
	for _, source := range run.Contributions {
		for _, item := range source.Bands {
			if item.BandHz == band {
				present = true
				total += algorithm.DBToRelativeEnergy(item.PredictedDB)
			}
		}
	}
	if !present {
		return 0, false
	}
	return algorithm.RelativeEnergyToDB(total), true
}

func buildTopBandDeltas(base, other dto.AttributionRunResponse, limit int) []dto.AttributionBandDelta {
	deltas := make([]dto.AttributionBandDelta, 0, len(constants.OctaveBands))
	for _, band := range constants.OctaveBands {
		baseLevel, baseOK := aggregateBandLevel(base, band)
		otherLevel, otherOK := aggregateBandLevel(other, band)
		if !baseOK || !otherOK {
			continue
		}
		difference := roundFloat(otherLevel-baseLevel, 3)
		deltas = append(deltas, dto.AttributionBandDelta{
			BandHz: band, BaseLevelDB: roundFloat(baseLevel, 3), OtherLevelDB: roundFloat(otherLevel, 3),
			DeltaDB: difference, AbsDeltaDB: math.Abs(difference),
		})
	}
	sort.SliceStable(deltas, func(i, j int) bool {
		if deltas[i].AbsDeltaDB != deltas[j].AbsDeltaDB {
			return deltas[i].AbsDeltaDB > deltas[j].AbsDeltaDB
		}
		return deltas[i].BandHz < deltas[j].BandHz
	})
	if len(deltas) > limit {
		deltas = deltas[:limit]
	}
	return deltas
}

func measurementChecksums(inputs []algorithm.MeasurementInput) []string {
	checksums := make([]string, 0, len(inputs))
	for _, input := range inputs {
		checksums = append(checksums, input.Checksum)
	}
	return checksums
}

func roundFloat(value float64, places int) float64 {
	formatted := fmt.Sprintf("%.*f", places, value)
	var rounded float64
	fmt.Sscanf(formatted, "%f", &rounded)
	return rounded
}
