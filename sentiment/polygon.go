package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/iter"
	"github.com/polygon-io/client-go/rest/models"
)

type Polygon struct {
	client *polygon.Client
}

func NewPolygon() (*Polygon, error) {
	api_key, is_present := os.LookupEnv("POLYGON_API_KEY")

	if !is_present {
		return nil, fmt.Errorf("Missing env var POLYGON_API_KEY")
	}

	client := polygon.New(api_key)

	return &Polygon{
		client: client,
	}, nil
}

func (p *Polygon) Overview(ctx context.Context, ticker string) (string, error) {
	params := models.GetTickerDetailsParams{
		Ticker: ticker,
	}

	res, err := p.client.GetTickerDetails(ctx, &params)
	if err != nil {
		return "", fmt.Errorf("Failed to get ticker overview: %v", err)
	}

	jsonBytes, err := json.Marshal(res)
	if err != nil {
		return "", fmt.Errorf("Failed to deserialize ticker overview: %v", err)
	}

	return string(jsonBytes), nil
}

func (p *Polygon) News(ctx context.Context, ticker string) *iter.Iter[models.TickerNews] {
	sort := models.Sort("published_utc")
	order := models.Order("asc")
	limit := 1
	twelveHoursAgo := models.Millis(time.Now().Add(-72 * time.Hour))

	params := models.ListTickerNewsParams{
		TickerEQ:        &ticker,
		PublishedUtcGTE: &twelveHoursAgo,
		Sort:            &sort,
		Order:           &order,
		Limit:           &limit,
	}

	return p.client.ListTickerNews(ctx, &params)
}
