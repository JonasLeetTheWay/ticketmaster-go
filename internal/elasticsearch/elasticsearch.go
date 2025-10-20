package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/JonasLeetTheWay/ticketmaster-go/internal/config"
	"github.com/JonasLeetTheWay/ticketmaster-go/internal/models"
)

type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(cfg *config.Config) (*Client, error) {
	client := &Client{
		baseURL: cfg.ElasticsearchURL,
		client:  &http.Client{},
	}

	// Test connection
	if err := client.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping Elasticsearch: %w", err)
	}

	// Create index if it doesn't exist
	if err := client.CreateIndex(); err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	return client, nil
}

func (c *Client) Ping() error {
	resp, err := c.client.Get(c.baseURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) CreateIndex() error {
	indexName := "events"

	// Check if index exists
	checkURL := fmt.Sprintf("%s/%s", c.baseURL, indexName)
	resp, err := c.client.Head(checkURL)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil // Index already exists
	}

	// Create index with mapping
	mapping := `{
		"mappings": {
			"properties": {
				"id": {"type": "integer"},
				"venueId": {"type": "integer"},
				"performerId": {"type": "integer"},
				"name": {"type": "text", "analyzer": "standard"},
				"description": {"type": "text", "analyzer": "standard"},
				"date": {"type": "date"},
				"venue": {"type": "text", "analyzer": "standard"},
				"performer": {"type": "text", "analyzer": "standard"},
				"genre": {"type": "keyword"},
				"location": {"type": "text", "analyzer": "standard"},
				"minPrice": {"type": "float"},
				"maxPrice": {"type": "float"},
				"availableTickets": {"type": "integer"}
			}
		}
	}`

	createURL := fmt.Sprintf("%s/%s", c.baseURL, indexName)
	req, err := http.NewRequest("PUT", createURL, strings.NewReader(mapping))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create index: %s", string(body))
	}

	return nil
}

func (c *Client) IndexEvent(event *models.ElasticsearchEvent) error {
	indexName := "events"

	// Convert to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_doc/%d?refresh=true", c.baseURL, indexName, event.ID)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(eventJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to index event: %s", string(body))
	}

	return nil
}

func (c *Client) SearchEvents(query map[string]interface{}) ([]models.ElasticsearchEvent, error) {
	indexName := "events"

	// Convert query to JSON
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	url := fmt.Sprintf("%s/%s/_search", c.baseURL, indexName)
	req, err := http.NewRequest("POST", url, bytes.NewReader(queryJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: %s", string(body))
	}

	// Parse response
	var searchResponse struct {
		Hits struct {
			Hits []struct {
				Source models.ElasticsearchEvent `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	events := make([]models.ElasticsearchEvent, len(searchResponse.Hits.Hits))
	for i, hit := range searchResponse.Hits.Hits {
		events[i] = hit.Source
	}

	return events, nil
}

func (c *Client) DeleteEvent(eventID uint) error {
	indexName := "events"

	url := fmt.Sprintf("%s/%s/_doc/%d?refresh=true", c.baseURL, indexName, eventID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete event: %s", string(body))
	}

	return nil
}

func (c *Client) UpdateEvent(event *models.ElasticsearchEvent) error {
	return c.IndexEvent(event) // Elasticsearch treats update as index
}

// BuildSearchQuery constructs an Elasticsearch query from search parameters
func BuildSearchQuery(term, location, eventType, date string) map[string]interface{} {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{},
			},
		},
		"size": 50,
	}

	boolQuery := query["query"].(map[string]interface{})["bool"].(map[string]interface{})
	mustClauses := boolQuery["must"].([]map[string]interface{})

	// Text search across name, description, performer, venue
	if term != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  term,
				"fields": []string{"name^2", "description", "performer^1.5", "venue"},
			},
		})
	}

	// Location filter
	if location != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"match": map[string]interface{}{
				"location": location,
			},
		})
	}

	// Genre/type filter
	if eventType != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"term": map[string]interface{}{
				"genre": eventType,
			},
		})
	}

	// Date filter
	if date != "" {
		mustClauses = append(mustClauses, map[string]interface{}{
			"range": map[string]interface{}{
				"date": map[string]interface{}{
					"gte": date,
				},
			},
		})
	}

	boolQuery["must"] = mustClauses

	return query
}
