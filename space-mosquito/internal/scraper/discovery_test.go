package scraper

import (
	"encoding/json"
	"testing"
)

func TestParseVersionResponse(t *testing.T) {
	// The sample response from the user
	jsonData := `{
  "results": [
    {
      "id": "515393957",
      "type": "page",
      "status": "current",
      "title": "Schnittstellen",
      "version": {
        "number": 11
      },
      "_links": {
        "webui": "/spaces/SK/pages/515393957/Schnittstellen"
      }
    },
    {
      "id": "533664653",
      "type": "page",
      "title": "Stempelkarte Home",
      "version": {
        "number": 3
      },
      "_links": {
        "webui": "/spaces/SK/pages/533664653/Stempelkarte+Home"
      }
    }
  ]
}`

	var result struct {
		Results []struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Version struct {
				Number int `json:"number"`
			} `json:"version"`
			Links struct {
				Webui string `json:"webui"`
			} `json:"_links"`
		} `json:"results"`
	}

	err := json.Unmarshal([]byte(jsonData), &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if len(result.Results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(result.Results))
	}

	if result.Results[0].Version.Number != 11 {
		t.Errorf("Expected version 11 for first item, got %d", result.Results[0].Version.Number)
	}

	if result.Results[1].Version.Number != 3 {
		t.Errorf("Expected version 3 for second item, got %d", result.Results[1].Version.Number)
	}
}
