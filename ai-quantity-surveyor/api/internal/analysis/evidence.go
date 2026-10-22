package analysis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
)

var roomEvidenceFields = map[string]bool{"length": true, "width": true, "wallHeight": true}
var openingEvidenceFields = map[string]bool{"width": true, "height": true}

func cloneEvidence(evidence Evidence) Evidence {
	if evidence.Region != nil {
		region := *evidence.Region
		evidence.Region = &region
	}
	if evidence.Confidence != nil {
		confidence := *evidence.Confidence
		evidence.Confidence = &confidence
	}
	return evidence
}

func cloneEvidenceMap(evidence map[string]Evidence) map[string]Evidence {
	if evidence == nil {
		return nil
	}
	result := make(map[string]Evidence, len(evidence))
	for field, value := range evidence {
		result[field] = cloneEvidence(value)
	}
	return result
}

func validEvidenceRegion(region *Bounds) bool {
	if region == nil {
		return true
	}
	for _, value := range []float64{region.X, region.Y, region.Width, region.Height} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return region.X >= 0 && region.Y >= 0 && region.Width > 0 && region.Height > 0 && region.X+region.Width <= 1 && region.Y+region.Height <= 1
}

func NormalizeEngineEvidence(candidate *Candidate) {
	candidate.ReviewIssues = nil
	clean := func(entity string, field string, evidence Evidence) (Evidence, bool) {
		if !validEvidenceRegion(evidence.Region) || (evidence.Confidence != nil && validateConfidence(*evidence.Confidence) != nil) {
			candidate.ReviewIssues = append(candidate.ReviewIssues, ReviewIssue{EntityID: entity, Field: field, Code: "InvalidEvidence", Message: "Source evidence is invalid and was excluded."})
			return Evidence{}, false
		}
		if candidate.Source == nil || (evidence.AnalysisID != "" && evidence.AnalysisID != candidate.ID) || (evidence.FloorPlanID != "" && evidence.FloorPlanID != candidate.FloorPlanID) || (evidence.PageNumber != 0 && evidence.PageNumber != 1 && evidence.PageNumber != candidate.Source.PageNumber) {
			candidate.ReviewIssues = append(candidate.ReviewIssues, ReviewIssue{EntityID: entity, Field: field, Code: "SourceMismatch", Message: "Evidence does not belong to the processed drawing page."})
			return Evidence{}, false
		}
		evidence.AnalysisID, evidence.FloorPlanID = candidate.ID, candidate.FloorPlanID
		evidence.PageNumber, evidence.Origin = candidate.Source.PageNumber, "engine"
		return evidence, true
	}
	cleanMap := func(id string, evidence map[string]Evidence, allowed map[string]bool) map[string]Evidence {
		result := make(map[string]Evidence)
		fields := make([]string, 0, len(evidence))
		for field := range evidence {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		for _, field := range fields {
			if !allowed[field] {
				candidate.ReviewIssues = append(candidate.ReviewIssues, ReviewIssue{EntityID: id, Field: field, Code: "UnsupportedEvidence", Message: "Evidence field is not supported and was excluded."})
				continue
			}
			if normalized, ok := clean(id, field, evidence[field]); ok {
				result[field] = normalized
			}
		}
		return result
	}
	for index := range candidate.Rooms {
		candidate.Rooms[index].Evidence = cleanMap(candidate.Rooms[index].ID, candidate.Rooms[index].Evidence, roomEvidenceFields)
	}
	for _, openings := range [][]Opening{candidate.Doors, candidate.Windows} {
		for index := range openings {
			openings[index].Evidence = cleanMap(openings[index].ID, openings[index].Evidence, openingEvidenceFields)
		}
	}
	for index := range candidate.Features {
		feature := &candidate.Features[index]
		feature.Origin, feature.Disposition = "engine", "pending"
		if feature.Evidence != nil {
			if evidence, ok := clean(feature.ID, "feature", *feature.Evidence); ok {
				feature.Evidence = &evidence
			} else {
				feature.Evidence = nil
			}
		}
	}
}

func confirmationAudit(original Candidate, confirmed *Candidate) error {
	confirmed.Corrections = nil
	confirmed.ReviewIssues = append([]ReviewIssue(nil), original.ReviewIssues...)
	record := func(id, field string, before, after any, origin string) {
		if reflect.DeepEqual(before, after) {
			return
		}
		previous, _ := json.Marshal(before)
		current, _ := json.Marshal(after)
		if bytes.Equal(previous, current) {
			return
		}
		confirmed.Corrections = append(confirmed.Corrections, Correction{EntityID: id, Field: field, OriginalValue: previous, ConfirmedValue: current, Origin: origin})
	}
	validateManual := func(id string, fields map[string]Evidence, allowed map[string]bool) error {
		for field, evidence := range fields {
			if !allowed[field] || evidence.Origin != "manual" || confirmed.Source == nil || evidence.AnalysisID != original.ID || evidence.FloorPlanID != original.FloorPlanID || evidence.PageNumber != confirmed.Source.PageNumber || !validEvidenceRegion(evidence.Region) || evidence.Confidence != nil {
				return fmt.Errorf("entity %q field %q has invalid manual source evidence", id, field)
			}
		}
		return nil
	}
	confirmedEvidence := func(id string, before, submitted map[string]Evidence, allowed map[string]bool) (map[string]Evidence, error) {
		result := cloneEvidenceMap(before)
		if result == nil && len(submitted) > 0 {
			result = make(map[string]Evidence)
		}
		for field, evidence := range submitted {
			if evidence.Origin != "manual" {
				continue
			}
			manual := map[string]Evidence{field: evidence}
			if err := validateManual(id, manual, allowed); err != nil {
				return nil, err
			}
			result[field] = cloneEvidence(evidence)
			var originalEvidence any
			if original, found := before[field]; found {
				originalEvidence = original
			}
			record(id, field+"Evidence", originalEvidence, evidence, "user")
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil
	}
	roomByID := make(map[string]Room)
	for _, room := range original.Rooms {
		roomByID[room.ID] = room
	}
	remainingRooms := make(map[string]bool)
	for index := range confirmed.Rooms {
		room := &confirmed.Rooms[index]
		remainingRooms[room.ID] = true
		previous, exists := roomByID[room.ID]
		if exists {
			var err error
			room.Evidence, err = confirmedEvidence(room.ID, previous.Evidence, room.Evidence, roomEvidenceFields)
			if err != nil {
				return err
			}
			room.Confidence, room.Uncertainty = previous.Confidence, previous.Uncertainty
		} else if err := validateManual(room.ID, room.Evidence, roomEvidenceFields); err != nil {
			return err
		}
		record(room.ID, "name", previous.Name, room.Name, "user")
		record(room.ID, "length", previous.Length, room.Length, "user")
		record(room.ID, "width", previous.Width, room.Width, "user")
		origin := "user"
		for _, field := range room.Assumptions {
			if field == "wallHeight" {
				origin = "configured-default"
			}
		}
		record(room.ID, "wallHeight", previous.WallHeight, room.WallHeight, origin)
		record(room.ID, "bounds", previous.Bounds, room.Bounds, "user")
	}
	for _, room := range original.Rooms {
		if !remainingRooms[room.ID] {
			record(room.ID, "removed", room, nil, "user")
		}
	}
	auditOpenings := func(previous, current []Opening) error {
		byID := make(map[string]Opening)
		for _, opening := range previous {
			byID[opening.ID] = opening
		}
		remaining := make(map[string]bool)
		for index := range current {
			opening := &current[index]
			remaining[opening.ID] = true
			before, exists := byID[opening.ID]
			if exists {
				var err error
				opening.Evidence, err = confirmedEvidence(opening.ID, before.Evidence, opening.Evidence, openingEvidenceFields)
				if err != nil {
					return err
				}
				opening.Confidence, opening.Uncertainty = before.Confidence, before.Uncertainty
			} else if err := validateManual(opening.ID, opening.Evidence, openingEvidenceFields); err != nil {
				return err
			}
			record(opening.ID, "roomId", before.RoomID, opening.RoomID, "user")
			record(opening.ID, "width", before.Width, opening.Width, "user")
			record(opening.ID, "height", before.Height, opening.Height, "user")
		}
		for _, opening := range previous {
			if !remaining[opening.ID] {
				record(opening.ID, "removed", opening, nil, "user")
			}
		}
		return nil
	}
	if err := auditOpenings(original.Doors, confirmed.Doors); err != nil {
		return err
	}
	if err := auditOpenings(original.Windows, confirmed.Windows); err != nil {
		return err
	}
	featureByID := make(map[string]Feature)
	for _, feature := range original.Features {
		featureByID[feature.ID] = feature
	}
	reviewedFeatures := make(map[string]bool)
	for index := range confirmed.Features {
		feature := &confirmed.Features[index]
		reviewedFeatures[feature.ID] = true
		before, exists := featureByID[feature.ID]
		if feature.Disposition != "acknowledged" && feature.Disposition != "removed" {
			return fmt.Errorf("feature %q requires an explicit review disposition", feature.ID)
		}
		if exists {
			feature.Evidence, feature.Confidence, feature.Origin = before.Evidence, before.Confidence, before.Origin
		} else {
			feature.Origin, feature.Confidence = "manual", nil
			if feature.Evidence != nil {
				if err := validateManual(feature.ID, map[string]Evidence{"feature": *feature.Evidence}, map[string]bool{"feature": true}); err != nil {
					return err
				}
			}
		}
		record(feature.ID, "feature", before, *feature, "user")
	}
	for _, feature := range original.Features {
		if !reviewedFeatures[feature.ID] {
			return fmt.Errorf("feature %q requires explicit acknowledgement or removal", feature.ID)
		}
	}
	return nil
}
