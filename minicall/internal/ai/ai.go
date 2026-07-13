package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Effortful-lion/agent-study/minicall/internal"
)

type Deepseek struct {
	Name string
}

func NewDeepseek() *Deepseek {
	return &Deepseek{
		Name: "Deepseek",
	}
}

func (d Deepseek) ModelName() string {
	return d.Name
}

func Invoke(model string, baseurl string, apikey string, question string) {
	// 3. invoke the model
	payload := internal.ChatRequest{
		Model: model,
		Messages: []internal.Message{
			{Role: "user", Content: question},
		},
		Stream: false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		baseurl+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apikey)
	// 4. return not stream
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		panic("unexpected status code: " + resp.Status)
	}

	// solve the response
	var result internal.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatal(err)
	}
	if len(result.Choices) == 0 {
		log.Fatal("模型响应中没有 choices")
	}
	if result.Usage.PromptTokens == 0 || result.Usage.CompletionTokens == 0 || result.Usage.TotalTokens == 0 {
		log.Fatal("模型响应中 usage 未取到")
	}
	fmt.Printf(
		"token: input=%d output=%d total=%d\n",
		result.Usage.PromptTokens,
		result.Usage.CompletionTokens,
		result.Usage.TotalTokens,
	)
	fmt.Println(result.Choices[0].Message.Content)
}
