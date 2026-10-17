package scraper

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/ArkanNajem/feedsanctum/internal/database"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func FetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "feedsanctum")
	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	resData, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	rssFeedData := &RSSFeed{}
	if err := xml.Unmarshal(resData, rssFeedData); err != nil {
		return nil, err
	}
	rssFeedData.Channel.Title = html.UnescapeString(rssFeedData.Channel.Title)
	rssFeedData.Channel.Description = html.UnescapeString(rssFeedData.Channel.Description)
	for i := range rssFeedData.Channel.Item {
		rssFeedData.Channel.Item[i].Title = html.UnescapeString(rssFeedData.Channel.Item[i].Title)
		rssFeedData.Channel.Item[i].Description = html.UnescapeString(rssFeedData.Channel.Item[i].Description)
	}
	return rssFeedData, nil
}

func ScrapeFeed(ctx context.Context, DB *database.Queries, feedURL string, feedID uuid.UUID) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in scraper for feed ID: %s, URL: %s: %v\n%s", feedID, feedURL, r, debug.Stack())
			err = fmt.Errorf("scraper panic: %v", r)
		}
	}()

	var rssDataFeed *RSSFeed
	rssDataFeed, err = FetchFeed(ctx, feedURL)
	if err != nil {
		return err
	}
	// Mark fetched before processing so a single broken feed doesn't
	// get retried until next batch.
	if err = DB.MarkFeedFetched(ctx, feedID); err != nil {
		return err
	}

	postURLs, fetchURLErr := DB.GetPostsURLsByFeedID(ctx, feedID)
	if fetchURLErr != nil {
		log.Printf("failed to get posts urls: %v", fetchURLErr)
	}
	postURLsMap := map[string]struct{}{}

	for _, postURL := range postURLs {
		postURLsMap[postURL] = struct{}{} // storing all posts for that feed ID so we can check if it exists
	}
	var created, skipped, failed int

	for _, item := range rssDataFeed.Channel.Item {

		if _, exists := postURLsMap[item.Link]; exists {
			skipped++
			continue
		}

		itemNullDescrip := sql.NullString{
			String: item.Description,
			Valid:  item.Description != "",
		}
		itemNullPub, parseErr := ParsePubTime(item.PubDate)
		if parseErr != nil {
			log.Printf("%v", parseErr)
		}

		_, createPostErr := DB.CreatePost(ctx, database.CreatePostParams{
			Title:       item.Title,
			Url:         item.Link,
			Description: itemNullDescrip,
			FeedID:      feedID,
			PublishedAt: itemNullPub,
		})
		if createPostErr != nil {
			var pqErr *pq.Error
			if errors.As(createPostErr, &pqErr) && pqErr.Code == "23505" {
				log.Printf("unique violation on constraint: post already exist id: %s", feedID)
				skipped++
				continue
			}
			log.Printf("failed to create post: %v, url: %s", createPostErr, item.Link)
			failed++
			continue
		}
		created++

	}
	log.Printf("scraped feed %s: %d created, %d skipped, %d failed", feedID, created, skipped, failed)
	return nil
}

func StartScraping(ctx context.Context, DB *database.Queries, tick time.Duration) {
	var wg sync.WaitGroup
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		feeds, err := DB.GetNextFeedsToFetch(ctx, 2)
		if err != nil {
			log.Printf("failed to fetch feeds to scrape: %v", err)
			continue
		}
		for _, feed := range feeds {
			wg.Add(1)
			go func(feedURL string, feedID uuid.UUID) {
				defer wg.Done()
				scrapeCTX, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				if err := ScrapeFeed(scrapeCTX, DB, feedURL, feedID); err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						log.Printf("scrape timedout after 30s: feed ID: %s URL: %s", feedID, feedURL)
					} else if errors.Is(err, context.Canceled) {
						log.Printf("scrape cancelled feed ID: %s URL: %s", feedID, feedURL)
					} else {
						log.Printf("failed to scrape feed. Feed ID: %s, URL: %s, Error: %v", feedID, feedURL, err)
					}

				}

			}(feed.Url, feed.ID)

		}
		wg.Wait()
	}

}

func ParsePubTime(rawLayout string) (sql.NullTime, error) {
	if rawLayout == "" {
		return sql.NullTime{}, nil
	}
	layouts := []string{time.RFC1123, time.RFC1123Z, time.RFC3339}
	for _, layout := range layouts {
		parsedTime, err := time.Parse(layout, rawLayout)
		if err == nil {
			return sql.NullTime{
				Time:  parsedTime,
				Valid: true,
			}, nil

		}
	}
	return sql.NullTime{}, fmt.Errorf("failed to find time layour for post,date: %s", rawLayout)
}
