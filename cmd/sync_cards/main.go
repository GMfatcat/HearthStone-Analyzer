package main

import (
	"context"
	"log"

	"hearthstone-analyzer/internal/app"
	"hearthstone-analyzer/internal/cards"
)

func main() {
	cfg := app.LoadConfig()
	runtime, err := app.Bootstrap(context.Background(), cfg)
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	defer func() {
		if err := runtime.DB.Close(); err != nil {
			log.Printf("database close error: %v", err)
		}
	}()

	source := cards.NewHearthstoneJSONSource(cfg.CardsSourceURL, cfg.CardsLocale, nil)
	service := cards.NewSyncService(runtime.Repositories.Cards, source)

	summary, err := service.Sync(context.Background())
	if err != nil {
		log.Fatalf("sync cards failed: %v", err)
	}

	log.Printf("cards sync complete: upserted=%d source=%s locale=%s", summary.CardsUpserted, cfg.CardsSourceURL, cfg.CardsLocale)
}
