// Probe utility: list publisher models available to this Vertex project,
// filtered to the Gemini family. Run once to discover the real Gemini 3.x
// model IDs, then update cmd/spike/main.go and docs accordingly.
//
// Run: GOOGLE_GENAI_USE_VERTEXAI=true GOOGLE_CLOUD_PROJECT=... GOOGLE_CLOUD_LOCATION=... \
//        go run ./cmd/probe-models
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"google.golang.org/genai"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  os.Getenv("GOOGLE_CLOUD_PROJECT"),
		Location: os.Getenv("GOOGLE_CLOUD_LOCATION"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: %v\n", err)
		os.Exit(1)
	}

	var names []string
	for page, err := range client.Models.All(ctx) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "list: %v\n", err)
			break
		}
		names = append(names, page.Name)
	}

	sort.Strings(names)
	gemini3 := []string{}
	other := []string{}
	for _, n := range names {
		short := lastSegment(n)
		if strings.Contains(short, "gemini-3") {
			gemini3 = append(gemini3, short)
		} else if strings.HasPrefix(short, "gemini") {
			other = append(other, short)
		}
	}

	fmt.Println("Gemini 3.x models available:")
	if len(gemini3) == 0 {
		fmt.Println("  (none — try a different LOCATION or check model garden access)")
	}
	for _, n := range gemini3 {
		fmt.Println("  ", n)
	}
	fmt.Println()
	fmt.Println("Other Gemini models (for reference):")
	for _, n := range other {
		fmt.Println("  ", n)
	}
}

func lastSegment(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
