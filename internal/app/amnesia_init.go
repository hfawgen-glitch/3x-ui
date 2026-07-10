package main

import (
	"log"

	"github.com/hfawgen-glitch/3x-ui/internal/config"
	"github.com/hfawgen-glitch/3x-ui/internal/database"
	"github.com/hfawgen-glitch/3x-ui/internal/service"
)

func initAmnesiaMode() (*database.MemoryDB, *service.AmnesiaService, error) {
	if !config.Amnesia.Enabled {
		return nil, nil, nil
	}

	memDB := database.NewMemoryDB()
	amnesiaService := service.NewAmnesiaService(memDB)

	if err := amnesiaService.Initialize(); err != nil {
		log.Printf("Failed to initialize amnesia mode: %v", err)
		return nil, nil, err
	}

	log.Println("Amnesia mode initialized")
	return memDB, amnesiaService, nil
}
