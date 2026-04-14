package discord

import (
	"context"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/redis/go-redis/v9"
	"globalsync/internal/queue"
)

// Ingestor handles the high-performance WebSocket connection designed strictly
// for listening and throwing raw messages into the NATS queue.
type Ingestor struct {
	Session *discordgo.Session
	RDB     *redis.Client
	NatsMgr *queue.NatsManager
}

func NewGatewayIngestor(token string, rdb *redis.Client, natsMgr *queue.NatsManager) (*Ingestor, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	// We only need the Guild Messages intent to read raw chat
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

	ingestor := &Ingestor{
		Session: dg,
		RDB:     rdb,
		NatsMgr: natsMgr,
	}

	dg.AddHandler(ingestor.OnMessageCreate)

	return ingestor, nil
}

func (i *Ingestor) Start() error {
	log.Println("Opening highly-available Gateway WebSocket pool...")
	return i.Session.Open()
}

func (i *Ingestor) Stop() {
	i.Session.Close()
}

// OnMessageCreate captures live raw messages and delegates scaling logic to DragonflyDB
func (i *Ingestor) OnMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages sent by ourselves or other bots to prevent infinity loops
	if m.Author.Bot {
		return
	}

	// 0. Privacy Middleware: Abort if the User has Opted Out of all interceptions.
	ctx := context.Background()
	userID := m.Author.ID

	optOutFlag, _ := i.RDB.Get(ctx, "user:"+userID+":optout").Result()
	if optOutFlag == "true" {
		return // Completely preserve their privacy natively
	}

	// 1. Ignorer Middleware: Validate raw text so we don't accidentally translate empty GIFs/Links/Emojis
	contentStr := m.Content
	linkRegex := regexp.MustCompile(`https?://[^\s]+`)
	emojiRegex := regexp.MustCompile(`<a?:.+?:\d+>`)
	
	stripped := linkRegex.ReplaceAllString(contentStr, "")
	stripped = emojiRegex.ReplaceAllString(stripped, "")
	if strings.TrimSpace(stripped) == "" {
		// The message is exclusively media or custom emojis. Ignore completely.
		return
	}

	channelID := m.ChannelID

	// 1. Maintain "Active Status" in DragonflyDB using an expiration ZSET (Score = Time)
	// Anyone who spikes in the last hour is considered "active" for translation targeting.
	activeKey := "channel:" + channelID + ":active"
	i.RDB.ZAdd(ctx, activeKey, redis.Z{Score: float64(time.Now().Unix()), Member: userID})
	
	maxScoreStr := strconv.FormatInt(time.Now().Add(-3*time.Minute).Unix(), 10)
	i.RDB.ZRemRangeByScore(ctx, activeKey, "-inf", maxScoreStr) // Prune old

	// 2. Fetch the Sender's native language to prevent translating Spanish -> Spanish
	senderLang, err := i.RDB.Get(ctx, "user:"+userID+":language").Result()
	if err != nil || senderLang == "" {
		return // If sender hasn't set a language, do nothing
	}

	// 3. Fetch all active users and map their REQUIRED target languages
	activeUsers, _ := i.RDB.ZRange(ctx, activeKey, 0, -1).Result()
	
	targetLanguages := make(map[string]bool)
	for _, uid := range activeUsers {
		if uid == userID {
			continue // Avoid translating to sender
		}
		userLang, _ := i.RDB.Get(ctx, "user:"+uid+":language").Result()
		
		if userLang != "" {
			normalizedUser := strings.ToLower(strings.TrimSpace(userLang))
			normalizedSender := strings.ToLower(strings.TrimSpace(senderLang))

			if normalizedUser != normalizedSender {
				targetLanguages[userLang] = true
			}
		}
	}

	// 4. Extract Server Nickname securely
	displayName := m.Author.Username
	if m.Member != nil && m.Member.Nick != "" {
		displayName = m.Member.Nick
	}

	langArray := make([]string, 0, len(targetLanguages))
	for lang := range targetLanguages {
		langArray = append(langArray, lang)
	}

	// 5. Dispatch 1 Unified Job to NATS
	if len(langArray) > 0 {
		// Instantly delete the original message natively in the background to prevent chat UI duplication
		go func() {
			err := i.Session.ChannelMessageDelete(m.ChannelID, m.ID)
			if err != nil {
				log.Printf("Warning: Failed to delete original message - Missing 'Manage Messages' permission? %v", err)
			}
		}()

		job := queue.TranslationJob{
			ChannelID:        m.ChannelID,
			OriginalUsername: displayName,
			AvatarURL:        m.Author.AvatarURL("128"),
			Content:          m.Content,
			TargetLanguages:  langArray,
			IsEnterprise:     false, 
		}
		
		log.Printf("Gateway: Throwing batched translation job [%s -> %v] into NATS queue", senderLang, langArray)
		i.NatsMgr.PublishJob(job)
	}
}
