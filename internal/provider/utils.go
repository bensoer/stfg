package provider

import (
	"encoding/json"
	"fmt"
	"math"
)

func groceryFlyerMatchesSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"matches": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"grocery_item":    map[string]any{"type": "string"},
						"flyer_item_id":   map[string]any{"type": "integer"},
						"flyer_item_name": map[string]any{"type": "string"},
					},
					"required":             []string{"grocery_item", "flyer_item_id", "flyer_item_name"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"matches"},
		"additionalProperties": false,
	}
}

func decodeGroceryFlyerMatches(providerName, raw string) ([]GroceryFlyerMatch, error) {
	var envelope struct {
		Matches json.RawMessage `json:"matches"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil || len(envelope.Matches) == 0 || string(envelope.Matches) == "null" {
		return nil, &ModelResponseError{Provider: providerName, Raw: raw}
	}

	var matches []GroceryFlyerMatch
	if err := json.Unmarshal(envelope.Matches, &matches); err != nil {
		return nil, &ModelResponseError{Provider: providerName, Raw: raw}
	}
	if matches == nil {
		matches = []GroceryFlyerMatch{}
	}
	return matches, nil
}

// toFloat32Vector narrows a float64 vector to float32. It rejects empty input
// and any NaN or Inf value, returning a ModelResponseError so the caller can
// distinguish bad payloads from transport failures.
func toFloat32Vector(providerName string, in []float64) ([]float32, error) {
	if len(in) == 0 {
		return nil, &ModelResponseError{Provider: providerName, Raw: "empty embedding vector"}
	}
	out := make([]float32, len(in))
	for i, v := range in {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, &ModelResponseError{Provider: providerName, Raw: fmt.Sprintf("non-finite embedding at index %d", i)}
		}
		out[i] = float32(v)
	}
	return out, nil
}
