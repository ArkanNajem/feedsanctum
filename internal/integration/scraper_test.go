package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ArkanNajem/feedsanctum/internal/scraper"
)

func TestFetchFeed(t *testing.T) {
	mockXML := `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
	<title>Go &amp; Tech Blog</title>
	<description>Latest news &amp; updates</description>
	<item>
		<title>Go 1.22 Released &amp; Features</title>
		<link>https://example.com/go122</link>
		<description>Exciting &amp; new features</description>
		<pubDate>Mon, 02 Jan 2006 15:04:05 MST</pubDate>
	</item>
</channel>
</rss>`

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockXML))
	}))
	defer testServer.Close()

	feed, err := scraper.FetchFeed(context.Background(), testServer.URL)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if feed.Channel.Title != "Go & Tech Blog" {
		t.Errorf("expected title 'Go & Tech Blog', got '%s'", feed.Channel.Title)
	}
	if feed.Channel.Description != "Latest news & updates" {
		t.Errorf("expected description 'Latest news & updates', got '%s'", feed.Channel.Description)
	}

	if len(feed.Channel.Item) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feed.Channel.Item))
	}

	item := feed.Channel.Item[0]
	if item.Title != "Go 1.22 Released & Features" {
		t.Errorf("expected item title 'Go 1.22 Released & Features', got '%s'", item.Title)
	}
	if item.Link != "https://example.com/go122" {
		t.Errorf("expected item link 'https://example.com/go122', got '%s'", item.Link)
	}
}

func TestParsePubTime(t *testing.T) {
	tests := []struct {
		name        string
		rawTime     string
		expectValid bool
		expectErr   bool
	}{
		{
			name:        "RFC1123 format",
			rawTime:     "Mon, 02 Jan 2006 15:04:05 MST",
			expectValid: true,
			expectErr:   false,
		},
		{
			name:        "RFC3339 format",
			rawTime:     "2006-01-02T15:04:05Z",
			expectValid: true,
			expectErr:   false,
		},
		{
			name:        "Empty string",
			rawTime:     "",
			expectValid: false,
			expectErr:   false,
		},
		{
			name:        "Invalid date format",
			rawTime:     "invalid-date-string",
			expectValid: false,
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := scraper.ParsePubTime(tt.rawTime)

			if (err != nil) != tt.expectErr {
				t.Errorf("expected err: %v, got: %v", tt.expectErr, err)
			}

			if res.Valid != tt.expectValid {
				t.Errorf("expected Valid field %v, got %v", tt.expectValid, res.Valid)
			}
		})
	}
}
func TestScrapeFeed(t *testing.T) {

	mockXML := `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
	<title>Test Feed</title>
	<description>Test Feed Description</description>
	<item>
		<title>Post 1</title>
		<link>https://example.com/post-1</link>
		<description>First post description</description>
		<pubDate>Mon, 02 Jan 2006 15:04:05 MST</pubDate>
	</item>
	<item>
		<title>Post 2</title>
		<link>https://example.com/post-2</link>
		<description>Second post description</description>
		<pubDate>Mon, 02 Jan 2006 15:04:05 MST</pubDate>
	</item>
</channel>
</rss>` // two items (posts)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockXML))
	}))
	defer mockServer.Close()

	ctx := context.Background()
	feed, err := testCfg.DB.CreateFeed(ctx, mockServer.URL)
	if err != nil {
		t.Fatalf("failed to create feed in DB: %v", err)
	}
	// Verify the two posts should get created (console log)
	err = scraper.ScrapeFeed(ctx, testCfg.DB, mockServer.URL, feed.ID)
	if err != nil {
		t.Fatalf("expected no error scraping feed, got %v", err)
	}

	posts, err := testCfg.DB.GetPostsURLsByFeedID(ctx, feed.ID)
	if err != nil {
		t.Fatalf("failed to fetch posts by feed ID: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts in DB, got %d", len(posts))
	}
	// Verify the two posts should get skipped (console log)
	err = scraper.ScrapeFeed(ctx, testCfg.DB, mockServer.URL, feed.ID)
	if err != nil {
		t.Fatalf("expected no error on duplicate scrape, got %v", err)
	}

	// Verify post count remains 2 (no duplicates added)
	postsAfterSecondScrape, err := testCfg.DB.GetPostsURLsByFeedID(ctx, feed.ID)
	if err != nil {
		t.Fatalf("failed to fetch posts by feed ID: %v", err)
	}
	if len(postsAfterSecondScrape) != 2 {
		t.Errorf("expected still 2 posts in DB after duplicate run, got %d", len(postsAfterSecondScrape))
	}
}
