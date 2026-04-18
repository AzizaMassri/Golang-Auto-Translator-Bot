package translator

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiService handles the Google Agent Dev Kit models in Go
type GeminiService struct {
	Client *genai.Client
}

var emojiTokenRegex = regexp.MustCompile(`(<a?:[a-zA-Z0-9_]+:\d+>|[\x{1F1E6}-\x{1F1FF}]{2}|[\x{1F300}-\x{1FAFF}\x{2600}-\x{27BF}](?:[\x{FE0F}\x{200D}\x{1F3FB}-\x{1F3FF}\x{1F300}-\x{1FAFF}\x{2600}-\x{27BF}])*)`)

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
	maskedText, placeholderMap := maskEmojiTokens(text)
	prompt := fmt.Sprintf("Translate strictly to %s. Map gamer slang organically. Keep all emojis exactly unchanged; never translate, replace, remove, or describe emoji meaning. Preserve Discord custom emoji tokens exactly (for example <:name:id> and <a:name:id>) with the same names/IDs. Placeholder tokens like [[EMJ_0]] must stay exactly unchanged and in the same positions. You must reply with ONLY the final translated text prefixed by the target flag emoji. Do NOT provide explanations, notes, or breakdowns under any circumstances. Text: %s", targetLang, maskedText)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty translation response")
	}

	// Extract standard text completion
	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	responseText = restoreEmojiTokens(responseText, placeholderMap)
	responseText = enforceEmojiPreservation(text, responseText)
	log.Printf("Translation Success [%s]", modelName)

	return responseText, nil
}

func maskEmojiTokens(input string) (string, map[string]string) {
	placeholders := map[string]string{}
	idx := 0
	masked := emojiTokenRegex.ReplaceAllStringFunc(input, func(token string) string {
		key := fmt.Sprintf("[[EMJ_%d]]", idx)
		placeholders[key] = token
		idx++
		return key
	})
	return masked, placeholders
}

func restoreEmojiTokens(input string, placeholders map[string]string) string {
	out := input
	for key, token := range placeholders {
		out = strings.ReplaceAll(out, key, token)
	}
	return out
}

func enforceEmojiPreservation(original, translated string) string {
	origTokens := emojiTokenRegex.FindAllString(original, -1)
	if len(origTokens) == 0 {
		return translated
	}

	transTokens := emojiTokenRegex.FindAllString(translated, -1)
	missing := []string{}
	searchStart := 0

	for _, tok := range origTokens {
		found := false
		for i := searchStart; i < len(transTokens); i++ {
			if transTokens[i] == tok {
				searchStart = i + 1
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, tok)
		}
	}

	if len(missing) == 0 {
		return translated
	}

	return strings.TrimSpace(translated + " " + strings.Join(missing, " "))
}
