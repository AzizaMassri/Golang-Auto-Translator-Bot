package queue

import (
	"context"
	"encoding/json"
	"log"

	"github.com/nats-io/nats.go"
)

type NatsManager struct {
	NC *nats.Conn
	JS nats.JetStreamContext
}

// TranslationJob defines the payload sent through JetStream
type TranslationJob struct {
	ChannelID        string   `json:"channel_id"`
	OriginalUsername string   `json:"original_username"`
	AvatarURL        string   `json:"avatar_url"`
	Content          string   `json:"content"`
	TargetLanguages  []string `json:"target_languages"`
	IsEnterprise     bool     `json:"is_enterprise"`
}

// Connect initializes the NATS connection and JetStream context
func (nm *NatsManager) Connect(url string) error {
	nc, err := nats.Connect(url)
	if err != nil {
		return err
	}
	nm.NC = nc

	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	nm.JS = js

	// Initialize the translation stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "TRANSLATIONS",
		Subjects: []string{"translate.>"},
	})
	if err != nil {
		log.Printf("Warning setting up JetStream: %v", err)
	}

	return nil
}

// PublishJob pushes a new Discord Interaction into the ultra-fast queue
func (nm *NatsManager) PublishJob(job TranslationJob) error {
	payload, _ := json.Marshal(job)
	
	// NATS JetStream publish
	_, err := nm.JS.Publish("translate.incoming", payload)
	if err != nil {
		return err
	}
	
	log.Printf("Published translation job to JetStream (user: %s)", job.OriginalUsername)
	return nil
}

// StartConsumer listens to JetStream and executes the bounded processing
func (nm *NatsManager) StartConsumer(ctx context.Context, handler func(job TranslationJob)) error {
	// NATS allows us to establish robust Push/Pull subscriptions that perfectly load-balance Goroutines
	
	sub, err := nm.JS.QueueSubscribe("translate.incoming", "translate_workers", func(msg *nats.Msg) {
		var job TranslationJob
		if err := json.Unmarshal(msg.Data, &job); err != nil {
			log.Printf("Decoding error: %v", err)
			msg.Nak()
			return
		}

		// Dispatch via goroutine to let NATS ingest instantly
		go func() {
			handler(job)
			msg.Ack()
		}()
	}, nats.MaxDeliver(3))

	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
	}()

	return nil
}
