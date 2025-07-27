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

const dateLayout = "2006-01-02T15:04:05Z"

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
	Chat  ChatSentimentResponse
	RanAt time.Time
}

type ChatSentimentResponse struct {
	Overview           string `json:"overview" jsonschema_description:"A very brief two or three response overview of the company"`
	TechnicalSentiment string `json:"technical_sentiment" jsonschema:"enum=bullish,enum=bearish,enum=neutral,enum=unknown" jsonschema_description:"The technical analysis driven sentiment on if this stock is worth watching for squeezes"`
	NewsSentiment      string `json:"news_sentiment" jsonschema:"enum=bullish,enum=bearish,enum=neutral,enum=unknown" jsonschema_description:"The news driven sentiment on if this stock is worth watching for squeezes"`
	SqueezePotential   string `json:"squeeze_potential" jsonschema:"enum=high,enum=medium,enum=low,enum=unknown" jsonschema_description:"The potential for an upcoming gamma/momentum squeeze today"`
	KnownCatalyst      string `json:"known_catalyst" jsonschema_description:"A short description on if there is a known catalyst, briefly describe what that catalyst is, and if there isn't a catalyst please say so."`
	Notes              string `json:"notes" jsonschema_description:"Any other brief important notes about this stock, or the market in general, for today, why you have given the sentiments you have, etc. Keep it brief as well."`
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
	You are a top-tier momentum day trader and technical analyst, trained to spot high-probability 
	intraday setups in small-cap stocks under extreme time pressure. Your reputation as a trader is 
	on the line. You specialize in reading price action, identifying potential short squeezes, 
	and evaluating whether momentum is likely to continue or fail. Your job is to assess each stock 
	with precision — analyzing volume trends, float size, technical levels (VWAP, key resistance, 
	high-of-day breaks, consolidation zones), and signs of short pressure (e.g. trap candles, 
	reclaim patterns, volume/price divergence). You think in terms of probability and edge. 
	You are biased toward trades with strong momentum and squeeze potential, but you're ruthless 
	about filtering out fakeouts and weak setups. This analysis must be actionable and reflect 
	real-time urgency. You are not writing for casual readers — you're writing for a focused, 
	fast-moving trader who needs a sharp, disciplined read on what's working right now.
    `

	userPrompt := fmt.Sprintf(`Ticker: %s
        Polygon Company Overview:
        %s
        
        Polygon News Description:
        %s

        Google News scraped headlines:
        %s

		Todays date (for news relevancy, don't depend on out-dated news!):
		%s
        
        Give your analysis:`, ticker, overview, newsString, gNewsResults, time.Now().Format(dateLayout))

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

	var chatSentimentResponse ChatSentimentResponse
	err = json.Unmarshal([]byte(aisb.String()), &chatSentimentResponse)
	if err != nil {
		sse.Error(err)
		return nil, err
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
			Chat:  chatSentimentResponse,
			RanAt: ranAt,
		}, nil
	}
}
