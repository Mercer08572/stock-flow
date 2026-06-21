package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/Mercer08572/stock-flow/internal/shared/config"
)

func main() {
	key := flag.String("key", "", "config key to print")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	switch *key {
	case "database_url":
		fmt.Print(cfg.DatabaseURL)
	case "app_env":
		fmt.Print(cfg.Environment)
	case "gin_mode":
		fmt.Print(cfg.GinMode)
	case "http_addr":
		fmt.Print(cfg.HTTPAddr)
	case "config_file":
		fmt.Print(cfg.ConfigFile)
	default:
		log.Fatalf("unsupported config key %q", *key)
	}
}
