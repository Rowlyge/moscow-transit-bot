package mosru

import (
"encoding/json"
"fmt"
"io"
"net/http"
"net/url"
"time"
)

const (
baseURL  = "https://apidata.mos.ru/v1"
pageSize = 500
)

type Client struct {
apiKey     string
httpClient *http.Client
}

func NewClient(apiKey string) *Client {
return &Client{
apiKey: apiKey,
httpClient: &http.Client{
Timeout: 30 * time.Second,
},
}
}

// FetchRows fetches all rows for a dataset from the /rows endpoint,
// paginating via $skip/$top until an empty page is returned.
// Rows are unmarshalled into rawRow (with Cells left as json.RawMessage)
// so callers can decode Cells into their own typed struct.
func (c *Client) FetchRows(datasetID int) ([]RawRow, error) {
var all []RawRow
skip := 0

for {
endpoint := fmt.Sprintf("%s/datasets/%d/rows", baseURL, datasetID)
q := url.Values{}
q.Set("api_key", c.apiKey)
q.Set("$skip", fmt.Sprintf("%d", skip))
q.Set("$top", fmt.Sprintf("%d", pageSize))

var page []RawRow
if err := c.getJSON(endpoint+"?"+q.Encode(), &page); err != nil {
return nil, fmt.Errorf("fetching rows for dataset %d (skip=%d): %w", datasetID, skip, err)
}

if len(page) == 0 {
break
}

all = append(all, page...)
skip += pageSize
}

return all, nil
}

// FetchFeatures fetches all features (rows with geometry) for a dataset
// from the /features endpoint, paginating the same way as FetchRows.
func (c *Client) FetchFeatures(datasetID int) ([]Feature, error) {
var all []Feature
skip := 0

for {
endpoint := fmt.Sprintf("%s/datasets/%d/features", baseURL, datasetID)
q := url.Values{}
q.Set("api_key", c.apiKey)
q.Set("$skip", fmt.Sprintf("%d", skip))
q.Set("$top", fmt.Sprintf("%d", pageSize))

var resp FeatureCollection
if err := c.getJSON(endpoint+"?"+q.Encode(), &resp); err != nil {
return nil, fmt.Errorf("fetching features for dataset %d (skip=%d): %w", datasetID, skip, err)
}

if len(resp.Features) == 0 {
break
}

all = append(all, resp.Features...)
skip += pageSize
}

return all, nil
}

func (c *Client) getJSON(url string, out interface{}) error {
resp, err := c.httpClient.Get(url)
if err != nil {
return fmt.Errorf("http request failed: %w", err)
}
defer resp.Body.Close()

body, err := io.ReadAll(resp.Body)
if err != nil {
return fmt.Errorf("reading response body: %w", err)
}

if resp.StatusCode != http.StatusOK {
return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
}

if err := json.Unmarshal(body, out); err != nil {
return fmt.Errorf("unmarshalling response: %w", err)
}

return nil
}
