package mosru

import (
"encoding/json"
"fmt"
"io"
"log"
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

func (c *Client) FetchRows(datasetID int) ([]RawRow, error) {
var all []RawRow
skip := 0

for {
endpoint := fmt.Sprintf("%s/datasets/%d/rows", baseURL, datasetID)
q := url.Values{}
q.Set("api_key", c.apiKey)
q.Set("$skip", fmt.Sprintf("%d", skip))
q.Set("$top", fmt.Sprintf("%d", pageSize))
fullURL := endpoint + "?" + q.Encode()

var page []RawRow
desc := fmt.Sprintf("dataset %d rows (skip=%d)", datasetID, skip)
err := withRetry(desc, func() error {
return c.getJSON(fullURL, &page)
})
if err != nil {
return nil, err
}

if len(page) == 0 {
break
}

all = append(all, page...)
log.Printf("  ...fetched page at skip=%d (%d rows so far)", skip, len(all))
skip += pageSize
}

return all, nil
}

func (c *Client) FetchFeatures(datasetID int) ([]Feature, error) {
var all []Feature
skip := 0

for {
endpoint := fmt.Sprintf("%s/datasets/%d/features", baseURL, datasetID)
q := url.Values{}
q.Set("api_key", c.apiKey)
q.Set("$skip", fmt.Sprintf("%d", skip))
q.Set("$top", fmt.Sprintf("%d", pageSize))
fullURL := endpoint + "?" + q.Encode()

var resp FeatureCollection
desc := fmt.Sprintf("dataset %d features (skip=%d)", datasetID, skip)
err := withRetry(desc, func() error {
return c.getJSON(fullURL, &resp)
})
if err != nil {
return nil, err
}

if len(resp.Features) == 0 {
break
}

all = append(all, resp.Features...)
log.Printf("  ...fetched page at skip=%d (%d features so far)", skip, len(all))
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
