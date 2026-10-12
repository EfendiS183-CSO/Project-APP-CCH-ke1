package main

import (
	"fmt"
	"log"

	"cch.local/backend/internal/platform/config"
	"cch.local/backend/internal/platform/database"
)

func main() {
	settings := config.Load()
	fmt.Printf("mysql configured: %v (host=%q port=%q user=%q passLen=%d db=%q)\n",
		settings.MySQL.Configured(), settings.MySQL.Host, settings.MySQL.Port,
		settings.MySQL.User, len(settings.MySQL.Password), settings.MySQL.Database)
	connection, err := database.OpenMySQL(settings.MySQL)
	if err != nil {
		log.Fatal("open: ", err)
	}
	defer connection.Close()
	if err := connection.Ping(); err != nil {
		log.Fatal("ping: ", err)
	}
	log.Print("ping ok")
}
