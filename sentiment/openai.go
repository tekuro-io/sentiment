package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
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

type ChatSentimentResponse struct {
	Overview           string `json:"overview" jsonschema_description:""`
	TechnicalSentiment string `json:"technical_sentiment" jsonschema:"enum=bullish,enum=bearish,enum=neutral" jsonschema_description:"The technical analysis driven sentiment on if this stock is worth watching for squeezes"`
	NewsSentiment      string `json:"news_sentiment" jsonschema:"enum=bullish,enum=bearish,enum=neutral" jsonschema_description:"The news driven sentiment on if this stock is worth watching for squeezes"`
	KnownCatalyst      string `json:"known_catalyst" jsonschema_description:"A short description on if there is a known catalyst, briefly describe what that catalyst is, and if there isn't a catalyst please say so."`
	Notes              string `json:"notes" jsonschema_description:"Any other brief important notes about this stock, or the market in general, for today."`
}

func GenerateSchema[T any]() interface{} {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

var ChatSentimentResponseSchema = GenerateSchema[ChatSentimentResponse]()

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

	systemPrompt := `
	You are an expert momentum day trader and financial analyst specializing in small-cap stocks. 
	Your job is to assess intraday opportunities by analyzing market conditions, and news catalysts. 
	You think like a trader: always looking for asymmetric risk/reward setups, especially in stocks
	with low float, high relative volume, and strong momentum. Your analysis should focus on why a 
	stock is moving, whether the move has potential to sustain, and what risks or red flags are present. 
	You pay close attention to key momentum factors like float rotation, volume surges, premarket gaps, 
	technical breakout levels, and sympathy plays. Focus entirely on insight, trading context, and 
	actionable thinking. Assume the user is a savvy trader who needs signal, not noise.
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
	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "chat_sentiment_response",
		Strict:      openai.Bool(true),
		Description: openai.String("Technical and news based analysis for a small cap stock ticker"),
		Schema:      ChatSentimentResponseSchema,
	}

	stream := o.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{JSONSchema: schemaParam},
		},
		Model: openai.ChatModelGPT4o,
	})
	defer stream.Close()

	acc := openai.ChatCompletionAccumulator{}
	var aisb strings.Builder
	first := false
	for stream.Next() {

		if !first {
			sse.ModelBegin()
			first = true
		}

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
