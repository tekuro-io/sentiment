package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/polygon-io/client-go/rest/models"
)

type OpenAi struct {
	client           openai.Client
	news_tool        *Polygon
	google_news_tool *GoogleScraper
}

func NewOpenAi() (*OpenAi, error) {
	api_key, is_present := os.LookupEnv("OPENAI_KEY")

	if !is_present {
		return nil, fmt.Errorf("Missing env var OPENAI_KEY")
	}

	client := openai.NewClient(
		option.WithAPIKey(api_key),
	)

	polygon, err := NewPolygon()
	if err != nil {
		return nil, err
	}

	return &OpenAi{
		client:           client,
		news_tool:        polygon,
		google_news_tool: NewGoogleScraper(),
	}, nil
}

type SentimentResponse struct {
	News  []models.TickerNews
	Chat  string
	RanAt time.Time
}

func (o *OpenAi) Sentiment(ctx context.Context, ticker string, sse *SSEWriter) (*SentimentResponse, error) {

	sse.Overview()
	overview, err := o.news_tool.Overview(ctx, ticker)
	if err != nil {
		sse.Error(err)
		return nil, err
	}

	sse.PNews()
	newsResults := o.news_tool.News(ctx, ticker)

	var newsList []models.TickerNews
	var sb strings.Builder
	for newsResults.Next() {
		newsItem := newsResults.Item()
		newsList = append(newsList, newsItem)
		jsonBytes, err := json.Marshal(newsItem.Description)
		if err != nil {
			sse.Error(fmt.Errorf("failed to deserialize ticker news: %v", err))
		}
		sb.Write(jsonBytes)
		sb.WriteByte('\n')
	}

	if err := newsResults.Err(); err != nil {
		sse.Error(fmt.Errorf("failed to deserialize ticker news: %v", err))
	}

	newsString := sb.String()

	sse.TickNews()
	sse.WriteNews(newsList)

	sse.GNews()
	gNewsResults, err := o.google_news_tool.getHeadlinesForTicker(ticker)
	if err != nil {
		sse.Error(err)
		return nil, err
	}

	systemPrompt := `You are a professional stock market news analyst. 
        Given news results about a stock, summarize:
        - What the company does (1-2 lines)
        - Today's main catalyst or news moving the stock
        - Could the stock experience a gamma squeeze or other technical price moves 
        - Sentiment (Bullish, Bearish, Neutral) and why
    `

	userPrompt := fmt.Sprintf(`Ticker: %s
        Polygon Company Overview:
        %s
        
        Polygon News Description:
        %s

        Google News scraped headlines:
        %s
        
        Give your analysis:`, ticker, overview, newsString, gNewsResults)

	sse.Model()
	stream := o.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Seed:  openai.Int(0),
		Model: openai.ChatModelGPT4o,
	})
	defer stream.Close()

	acc := openai.ChatCompletionAccumulator{}
	var aisb strings.Builder
	sse.ModelBegin()
	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			sse.WriteEvent(content)
			aisb.WriteString(content)
		}
	}

	if err := stream.Err(); err != nil {
		sse.Error(err)
		return nil, err
	} else {
		sse.RanAt()
		ranAt := time.Now()
		sse.WriteRanAt(ranAt)
		sse.Done()
		return &SentimentResponse{
			News:  newsList,
			Chat:  aisb.String(),
			RanAt: ranAt,
		}, nil
	}
}
