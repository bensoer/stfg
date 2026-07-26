package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	openrouter "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
)

type Client struct {
	sdk *openrouter.OpenRouter
}

func NewClient(apiKey string) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}

	sdk := openrouter.New(
		openrouter.WithSecurity(apiKey),
		openrouter.WithClient(&http.Client{Timeout: 3 * time.Minute}),
	)

	return &Client{
		sdk: sdk,
	}
}

func (c *Client) Send(prompt, model string) (*string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	res, err := c.sdk.Chat.Send(ctx, components.ChatRequest{
		Model: openrouter.Pointer(model),
		Messages: []components.ChatMessages{
			components.CreateChatMessagesUser(
				components.ChatUserMessage{
					Content: components.CreateChatUserMessageContentStr(prompt),
					Role:    components.ChatUserMessageRoleUser,
				},
			),
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errors.New("empty response from openrouter")
	}

	content, ok := res.ChatResult.Choices[0].GetMessage().Content.GetOrZero()
	if !ok {
		return nil, fmt.Errorf("unexpected response format: %v", res)
	} // Access the text content of the first choice

	return content.Str, nil

}
