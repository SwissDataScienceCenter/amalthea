package controller

import (
	"log"
	"os"
)

type config struct {
	SidecarsImage string
}

func NewConfigFromEnv() config {
	sessionImg := os.Getenv("SIDECARS_IMAGE")
	if sessionImg == "" {
		log.Fatalf("Could not find the %q environment variable", "SIDECARS_IMAGE")
	}
	return config{SidecarsImage: sessionImg}
}
