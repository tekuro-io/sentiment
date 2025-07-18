package sentiment

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)


type GoogleScraper struct {
    urlSupplier func(query string) string
}

func NewGoogleScraper() *GoogleScraper {
    return &GoogleScraper{
    	urlSupplier: func(query string) string {
            return fmt.Sprintf("https://news.google.com/search?q=%s", query)
        },
    }
}

type ArticleData struct {
    aTags []string
    timeTags []string
}

func (a ArticleData) String() string {
    return fmt.Sprintf("Headlines: %s\nTimes:%s\n",
        strings.Join(a.aTags, ", "),
        strings.Join(a.timeTags, ", "),
    )
}

func (s *GoogleScraper) getHeadlinesForTicker(ticker string) (string, error) {
    url := s.urlSupplier(ticker)

    resp, err := http.Get(url)
    if err != nil {
        return "", fmt.Errorf("Google news failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return "", fmt.Errorf("Google news bad status code: %v", err)
    }

    doc, err := goquery.NewDocumentFromReader(resp.Body)
    if err != nil {
        return "", fmt.Errorf("Failed to parse Google news response %v", err)
    }

    var articles []ArticleData
    doc.Find("article").Each(func(i int, article *goquery.Selection) {
        var aTags []string
        var timeTags []string
        article.Find("a").Each(func(_ int, a *goquery.Selection) {
            text := strings.TrimSpace(a.Text())
            if text != "" {
                aTags = append(aTags, text)
            }
        })

        article.Find("time").Each(func(_ int, t *goquery.Selection) {
            text := strings.TrimSpace(t.Text())
            if text != "" {
                timeTags = append(timeTags, text)
            }
        })

        articles = append(articles, ArticleData{
        	aTags:    aTags,
        	timeTags: timeTags,
        })
    })

    var articleStrings []string
    for _, article := range articles {
        articleStrings = append(articleStrings, article.String())
    }

    return strings.Join(articleStrings, "\n"), nil
}
