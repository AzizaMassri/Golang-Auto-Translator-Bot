package discord

import (
	"context"
	"log"
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

	ctx := context.Background()
	channelID := m.ChannelID
	userID := m.Author.ID

	// 1. Maintain "Active Status" in DragonflyDB using an expiration ZSET (Score = Time)
	// Anyone who spikes in the last hour is considered "active" for translation targeting.
	activeKey := "channel:" + channelID + ":active"
	i.RDB.ZAdd(ctx, activeKey, redis.Z{Score: float64(time.Now().Unix()), Member: userID})
	i.RDB.ZRemRangeByScore(ctx, activeKey, "0", float64(time.Now().Add(-1*time.Hour).Unix())) // Prune old

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
		if userLang != "" && userLang != senderLang {
			targetLanguages[userLang] = true
		}
	}

	// 4. Dispatch 1 Job per Target Language to the infinitely-scalable NATS pipeline
	for lang := range targetLanguages {
		job := queue.TranslationJob{
			ChannelID:        m.ChannelID,
			OriginalUsername: m.Author.Username,
			AvatarURL:        m.Author.AvatarURL("128"),
			Content:          m.Content,
			TargetLanguage:   lang,
			IsEnterprise:     false, // Can be pulled from Premium entitlement DB
		}
		
		log.Printf("Gateway: Throwing auto-translation job [%s -> %s] into NATS queue", senderLang, lang)
		i.NatsMgr.PublishJob(job)
	}
}
