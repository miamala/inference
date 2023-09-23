package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/sashabaranov/go-openai"
)

var (
	apiKey string
	prompt string
	//go:embed schema.json
	schema string
)

type Transaction struct {
	Amount   float64 `json:"amount"`
	Category string  `json:"category"`
	Type     string  `json:"type"`
}

func main() {
	apiKey = os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("api key not found")
	}

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.POST("/upload", upload)

	e.Logger.Fatal(e.Start(":1323"))
}

func upload(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return err
	}
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	path := file.Filename

	dst, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return err
	}

	transactionText, err := extractText(path)

	if err != nil {
		log.Fatal(err)
	}

	prompt = "You are a helpful assistant extracting a personal transaction from text. " +
		"From the given text, create a valid JSON representation that strictly follows this schema:\n\n" +
		schema + "\n\n" +
		"If no transaction can be found return an empty string. Omit null fields. Return JSON only. The JSON object:\n\n"

	transaction, err := extractTransaction(transactionText)
	if err != nil {
		log.Fatal(err)
	}

	buf, err := json.Marshal(transaction)
	if err != nil {
		log.Fatal(err)
	}

	return c.JSON(http.StatusOK, string(buf))
}

// use whisper to convert audio to text
func extractText(path string) (string, error) {
	c := openai.NewClient(apiKey)
	ctx := context.Background()

	req := openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: path,
	}

	resp, err := c.CreateTranscription(
		ctx,
		req,
	)
	if err != nil {
		return "", err
	}

	return resp.Text, nil
}

// Max 25MB, mp3, mp4, mpeg, mpga, m4a, wav, and webm
// extractTransaction sends text to GPT to extract transactions
func extractTransaction(s string) (*Transaction, error) {
	c := openai.NewClient(apiKey)
	ctx := context.Background()

	req := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: prompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: s,
			},
		},
		MaxTokens:   2000,
		Temperature: 1,
		Stop:        nil,
		N:           1,
	}

	resp, err := c.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	completion := resp.Choices[0].Message.Content

	fmt.Println(completion)

	trasaction := &Transaction{}
	json.Unmarshal([]byte(completion), trasaction)

	return trasaction, nil
}
