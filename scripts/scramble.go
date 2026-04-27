package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load the .env from the root (two levels up from /scripts)
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	plainSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	xorKey := os.Getenv("THAW_XOR_KEY")

	if plainSecret == "" || xorKey == "" {
		log.Fatal("Missing environment variables")
	}

	scrambled := make([]byte, len(plainSecret))
	for i := 0; i < len(plainSecret); i++ {
		scrambled[i] = plainSecret[i] ^ xorKey[i%len(xorKey)]
	}

	fmt.Printf("Generated Scrambled Hex: %s\n", hex.EncodeToString(scrambled))
}
