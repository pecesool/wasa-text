package main

import (
	"os"
)

type Configuration struct {
	HTTPAddr string
}

func loadConfiguration() Configuration {
	// Default
	addr := "0.0.0.0:3000"

	if p := os.Getenv("PORT"); p != "" {
		addr = "0.0.0.0:" + p
	}

	if a := os.Getenv("HTTP_ADDR"); a != "" {
		addr = a
	}

	return Configuration{
		HTTPAddr: addr,
	}
}
