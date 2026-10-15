// Package elastic 은 Elasticsearch REST API 를 직접 호출하는 얇은 클라이언트다.
//
// 공식 Go 클라이언트를 쓰지 않는 이유: 필요한 건 ES|QL(/_query)과 _search 두 엔드포인트뿐이고,
// 의존성과 버전 호환 부담을 지지 않으려는 것이다.
//
// ES|QL 은 8.15 에서 histogram 필드(APM 의 사전 집계 메트릭)를 읽지 못한다. 그래서 원본
// 트랜잭션은 ES|QL 로, 사전 집계 메트릭이 필요해지면 _search 집계로 조회한다.
package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// ESQLResult 는 ES|QL 의 표 형태 응답이다.
type ESQLResult struct {
	Columns []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"columns"`
	Values [][]any `json:"values"`
}

// Rows 는 각 행을 열 이름 → 값 맵으로 돌려준다.
func (r *ESQLResult) Rows() []map[string]any {
	rows := make([]map[string]any, 0, len(r.Values))
	for _, v := range r.Values {
		row := make(map[string]any, len(r.Columns))
		for i, c := range r.Columns {
			if i < len(v) {
				row[c.Name] = v[i]
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// ESQL 은 ES|QL 쿼리를 실행한다. params 는 쿼리 안의 ? 에 순서대로 들어간다.
func (c *Client) ESQL(ctx context.Context, query string, params ...any) (*ESQLResult, error) {
	body := map[string]any{"query": query}
	if len(params) > 0 {
		body["params"] = params
	}
	var out ESQLResult
	if err := c.do(ctx, http.MethodPost, "/_query", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Search 는 _search 를 실행하고 응답 원문을 out 으로 디코딩한다.
func (c *Client) Search(ctx context.Context, index string, body any, out any) error {
	return c.do(ctx, http.MethodPost, "/"+index+"/_search", body, out)
}

// Ping 은 인증된 상태로 ES 가 응답하는지 확인한다.
func (c *Client) Ping(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/", nil, nil)
}

// Error 는 ES 가 2xx 가 아닌 응답을 줬을 때의 에러다.
type Error struct {
	Status int
	Body   string
}

func (e *Error) Error() string {
	return fmt.Sprintf("elasticsearch %d: %s", e.Status, e.Body)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return &Error{Status: resp.StatusCode, Body: string(b)}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
