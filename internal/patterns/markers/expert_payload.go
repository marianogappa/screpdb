package markers

import "encoding/json"

// ExpertActual is the per-milestone payload entry. Position-aligned with
// Marker.Expert, so the reader walks both lists in lockstep — the static
// Key / TargetSecond / Tolerance live in code, only the resolved second
// (and "found" bit) need to round-trip through DB.
type ExpertActual struct {
	// Second is the replay second the milestone actually fired. Omitted from
	// JSON when Found is false to keep the payload small.
	Second int  `json:"second,omitempty"`
	Found  bool `json:"found"`
}

// ExpertPayload is the wrapper persisted to replay_events.payload for any
// build-order marker. Wrapper keeps the door open for additional BO-specific
// fields without breaking decode.
type ExpertPayload struct {
	ExpertActuals []ExpertActual `json:"expert_actuals"`
}

// EncodeExpertActuals collapses a list of ExpertResolution (one per
// Marker.Expert event, in declaration order) into the persisted JSON.
func EncodeExpertActuals(resolutions []ExpertResolution) (json.RawMessage, error) {
	actuals := make([]ExpertActual, len(resolutions))
	for i, r := range resolutions {
		actuals[i].Found = r.Found
		if r.Found {
			actuals[i].Second = r.ActualSecond
		}
	}
	return json.Marshal(ExpertPayload{ExpertActuals: actuals})
}

// DecodeExpertActuals parses a payload row. Returns nil when the payload
// has no expert_actuals key or fails to parse — caller decides what to do
// (with no fallback path, the caller treats nil as "all milestones missing"
// and renders accordingly).
func DecodeExpertActuals(payload []byte) []ExpertActual {
	if len(payload) == 0 {
		return nil
	}
	var wrapper ExpertPayload
	if err := json.Unmarshal(payload, &wrapper); err != nil {
		return nil
	}
	return wrapper.ExpertActuals
}
