package main

import (
	"os"
	"strconv"

	"github.com/beto/trading-agent/market-api/api"
	"github.com/beto/trading-agent/market-api/internal/okx"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	godotenv.Load("/app/.env")

	cfg := &okx.Config{
		APIKey:     mustEnv("OKX_API_KEY"),
		SecretKey:  mustEnv("OKX_SECRET_KEY"),
		Passphrase: mustEnv("OKX_PASSPHRASE"),
		IsDemo:     os.Getenv("OKX_DEMO") == "true",
	}

	leverage := 3
	if v := os.Getenv("LEVERAGE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			leverage = n
		}
	}

	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	client := okx.NewClient(cfg, log)
	defer client.Close()

	srv := api.NewServer(client, log, leverage)
	if err := srv.Start(addr); err != nil {
		log.WithError(err).Fatal("server stopped")
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		logrus.Fatalf("required env var %s is not set", key)
	}
	return v
}
