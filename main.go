package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

const (
	maxBodySize    = 50 * 1024 * 1024 // 50MB
	rateLimitDelay = 2 * time.Second
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please provide at least one event name as an argument.")
	}

	events := os.Args[1:] // Get all event names from command-line arguments

	for _, event := range events {
		downloadEventFiles(event)
	}
}

// downloadEventFiles handles the crawling and downloading of event data
func downloadEventFiles(event string) {
	if err := os.MkdirAll(event, os.ModePerm); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	collector := initializeCollector(event)

	// Start crawling the overview page
	startURL := fmt.Sprintf("https://%s.sched.com/overview", event)
	log.Printf("Starting to crawl %s\n", startURL)

	if err := collector.Visit(startURL); err != nil {
		log.Printf("Failed to visit talk overview: %v", err)
	}

	fmt.Print("\n")
}

// initializeCollector initializes and configures the Colly collector
func initializeCollector(event string) *colly.Collector {
	c := colly.NewCollector(
		colly.MaxBodySize(maxBodySize),
		// colly.Debugger(&debug.LogDebugger{}),
	)

	// Set rate limiting
	if err := c.Limit(&colly.LimitRule{
		DomainGlob:  "sched.com",
		Parallelism: 1,
		RandomDelay: rateLimitDelay,
	}); err != nil {
		log.Printf("Failed setting rate limiting: %v", err)
	}

	extensions.RandomUserAgent(c)

	// Set headers for requests
	c.OnRequest(func(r *colly.Request) {
		r.Ctx.Put("event", event)
	})

	// Define the first layer - overview links
	c.OnHTML("div.list-simple div.sched-container-inner a", func(e *colly.HTMLElement) {
		if err := e.Request.Visit(e.Attr("href")); err != nil {
			log.Printf("Failed to visit talk URL: %v", err)
		}
	})

	// Define the second layer - download links
	c.OnHTML("a.file-uploaded", func(e *colly.HTMLElement) {
		fileURL := e.Attr("href")
		log.Printf("Found file in %s", fileURL)
		// Visit the file URL to trigger the OnResponse callback
		if err := e.Request.Visit(fileURL); err != nil {
			log.Printf("Failed to visit file URL: %v", err)
		}
	})

	// Handle the response for file downloads
	c.OnResponse(func(r *colly.Response) {
		if !strings.Contains(r.Request.URL.Path, "hosted_files") {
			return
		}

		dirName := r.Request.Ctx.Get("event") // Retrieve the event name from the context
		fileName := strings.ReplaceAll(r.FileName(), "hosted_files_", "")
		filePath := fmt.Sprintf("%s/%s", dirName, fileName)
		log.Printf("Downloading file %s from %s", filePath, r.Request.URL)

		// Save the response body to a file
		if err := r.Save(filePath); err != nil {
			log.Printf("Failed to save file: %v", err)
		} else {
			log.Printf("Downloaded file: %s", filePath)
		}
	})

	return c
}
