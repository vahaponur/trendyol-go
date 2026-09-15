package trendyol

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ListOrdersStreamOptions filters packages by their last modification time.
// Keep the filters and size unchanged when continuing with NextCursor.
// The API always sorts by lastModifiedDate descending; it does not accept page.
type ListOrdersStreamOptions struct {
	Size                  int
	NextCursor            string
	PackageItemStatuses   string
	LastModifiedStartDate *time.Time
	LastModifiedEndDate   *time.Time
}

// StreamResponse describes the next page of an order stream.
// Pass NextCursor back unchanged while HasMore is true.
type StreamResponse struct {
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor"`
	Size       int    `json:"size"`
}

// ListStream fetches one page from getShipmentPackagesStream. A zero Size lets
// Trendyol use its default (50); the maximum is 200. The date window is at most
// 14 days within the last 3 months, or the last 14 days when dates are omitted.
func (s *orderService) ListStream(ctx context.Context, opts ListOrdersStreamOptions) ([]Order, *StreamResponse, error) {
	query := url.Values{}
	if opts.Size != 0 {
		query.Set("size", strconv.Itoa(opts.Size))
	}
	if opts.NextCursor != "" {
		query.Set("nextCursor", opts.NextCursor)
	}
	if opts.PackageItemStatuses != "" {
		query.Set("packageItemStatuses", opts.PackageItemStatuses)
	}
	if opts.LastModifiedStartDate != nil {
		query.Set("lastModifiedStartDate", strconv.FormatInt(opts.LastModifiedStartDate.UnixMilli(), 10))
	}
	if opts.LastModifiedEndDate != nil {
		query.Set("lastModifiedEndDate", strconv.FormatInt(opts.LastModifiedEndDate.UnixMilli(), 10))
	}
	var result struct {
		Content []Order `json:"content"`
		StreamResponse
	}
	err := s.client.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   s.client.resolve(EndpointGetOrdersStreamKey, s.client.sellerID),
		Query:  query,
		Result: &result,
	})
	if err != nil {
		return nil, nil, err
	}
	return result.Content, &result.StreamResponse, nil
}
