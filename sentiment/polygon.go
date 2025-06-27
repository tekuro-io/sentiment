package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	polygon "github.com/polygon-io/client-go/rest"
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

func (p *Polygon) News(ctx context.Context, ticker string) (string, error) {
	sort := models.Sort("published_utc")
	order := models.Order("asc")
	limit := 1
	params := models.ListTickerNewsParams{
		TickerEQ: &ticker,
		Sort:     &sort,
		Order:    &order,
		Limit:    &limit,
	}

    iter := p.client.ListTickerNews(ctx, &params)

    var sb strings.Builder
    for iter.Next() {
        jsonBytes, err := json.Marshal(iter.Item())
        if err != nil {
            return "", fmt.Errorf("Failed to deserialize ticker news: %v", err)
        }
        sb.Write(jsonBytes)
        sb.WriteByte('\n')
	}

    if err := iter.Err(); err != nil {
        return "", fmt.Errorf("Fetching ticker news failed: %v", err)
	}

    return sb.String(), nil
}
