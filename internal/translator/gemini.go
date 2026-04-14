package translator

import (
	"context"
	"fmt"
	"log"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiService handles the Google Agent Dev Kit models in Go
type GeminiService struct {
	Client *genai.Client
}

// NewGeminiService securely binds the API Key
func NewGeminiService(ctx context.Context, apiKey string) (*GeminiService, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	return &GeminiService{Client: client}, nil
}

// Translate routes dynamically to Flash 2.5 or Pro 3.1
func (g *GeminiService) Translate(ctx context.Context, text, targetLang string, isEnterprise bool) (string, error) {
	var modelName string
	if isEnterprise {
		// Premium latency, high detail Model
		modelName = "gemini-3.1-pro"
	} else {
		// High throughput, cost-effective Model
		modelName = "gemini-2.5-flash"
	}

	model := g.Client.GenerativeModel(modelName)
	prompt := fmt.Sprintf("Translate this strictly and literally to %s. However, if you detect gamer slang or abbreviations (like brb, gg, lol), accurately translate those specific slang terms to their appropriate %s equivalents while keeping the rest of the sentence literal. Prefix your response exclusively with the country flag emoji for that language, followed by a space, followed the translation. Keep formatting intact. Text: %s", targetLang, targetLang, text)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty translation response")
	}

	// Extract standard text completion
	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	log.Printf("Translation Success [%s]", modelName)

	return responseText, nil
}
