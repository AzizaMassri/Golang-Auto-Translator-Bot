package translator

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

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

// Transcribe transcribes audio from URL to text
func (g *GeminiService) Transcribe(ctx context.Context, audioURL string) (string, error) {
	// Download audio
	resp, err := http.Get(audioURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	model := g.Client.GenerativeModel("gemini-2.5-flash")
	part := genai.Blob{MIMEType: "audio/ogg", Data: audioData} // Assuming OGG, Discord uses OGG for voice

	respGem, err := model.GenerateContent(ctx, part, genai.Text("Transcribe this audio to text. Reply with only the transcription."))
	if err != nil {
		return "", err
	}

	if len(respGem.Candidates) == 0 || len(respGem.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty translation response")
	}

	transcription := fmt.Sprintf("%v", respGem.Candidates[0].Content.Parts[0])
	log.Printf("Transcription Success")

	return transcription, nil
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
	prompt := fmt.Sprintf("Translate strictly to %s. Map gamer slang organically. You must reply with ONLY the final translated text prefixed by the target flag emoji. Do NOT provide explanations, notes, or breakdowns under any circumstances. Text: %s", targetLang, text)

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
