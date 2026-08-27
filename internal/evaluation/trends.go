package evaluation

import "mural-conservation-gate/internal/domain"

func matchingControl(input Input) (domain.TrialObservation, bool) {
	for _, control := range input.Controls {
		if control.RoundNo == input.Current.RoundNo && control.ProtocolRevisionID == input.Current.ProtocolRevisionID {
			return control, true
		}
	}
	return domain.TrialObservation{}, false
}

func isTrialZone(zoneID string, controls []domain.TrialObservation) bool {
	for _, control := range controls {
		if control.ZoneID == zoneID {
			return false
		}
	}
	return true
}

func trendRising(history []domain.TrialObservation, current domain.TrialObservation, metric func(domain.TrialObservation) float64) bool {
	var previous *domain.TrialObservation
	for i := range history {
		candidate := &history[i]
		if candidate.ZoneID == current.ZoneID && candidate.ProtocolRevisionID == current.ProtocolRevisionID && candidate.BaselineRevisionID == current.BaselineRevisionID && candidate.RoundNo < current.RoundNo {
			if previous == nil || candidate.RoundNo > previous.RoundNo {
				previous = candidate
			}
		}
	}
	if previous == nil {
		return false
	}
	return metric(current) > metric(*previous)*1.2 && metric(current)-metric(*previous) > 0.2
}
