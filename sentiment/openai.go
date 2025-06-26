package sentiment

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type OpenAi struct {
	client openai.Client
}

func NewOpenAi() (*OpenAi, error) {
	api_key, is_present := os.LookupEnv("OPENAI_KEY")

	if !is_present {
		return nil, fmt.Errorf("Missing env var OPENAI_KEY")
	}

	client := openai.NewClient(
		option.WithAPIKey(api_key),
	)

	return &OpenAi{
		client: client,
	}, nil
}

func (o *OpenAi) Sentiment(ctx context.Context, ticker string, sse *SSEWriter) {

	stream := o.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(ticker),
		},
		Seed:  openai.Int(0),
		Model: openai.ChatModelGPT4o,
	})
    defer stream.Close()

    acc := openai.ChatCompletionAccumulator{}
    for stream.Next() {
        chunk := stream.Current()
	    acc.AddChunk(chunk)

	    if content, ok := acc.JustFinishedContent(); ok {
	    	log.Println("Content stream finished:", content)
	    }

	    // if using tool calls
	    if tool, ok := acc.JustFinishedToolCall(); ok {
	    	log.Println("Tool call stream finished:", tool.Index, tool.Name, tool.Arguments)
	    }

	    if refusal, ok := acc.JustFinishedRefusal(); ok {
	    	log.Println("Refusal stream finished:", refusal)
	    }

	    if len(chunk.Choices) > 0 {
            sse.WriteEvent(chunk.Choices[0].Delta.Content)
	    }
    }

    if err := stream.Err(); err != nil {
		sse.Error(err)
	} else {
        sse.Done()
    }
}
