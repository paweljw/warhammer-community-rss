package main

import (
	"context"
	"flag"
	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
	"github.com/gorilla/feeds"
	"github.com/robfig/cron/v3"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.93 Safari/537.36"

func EnclosureSize(url string) string {
	enclosureSize := "0"

	resp, subErr := http.Head(url)

	if subErr == nil && resp.StatusCode == http.StatusOK {
		enclosureSize = resp.Header.Get("Content-Length")
	}

	return enclosureSize
}

func WarhammerCommunityProcessing(ctx context.Context) error {
	node, err := dom.GetDocument().Do(ctx)
	if err != nil {
		return err
	}

	str, err := dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
	if err != nil {
		log.Fatal(err)
		return err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(str))
	if err != nil {
		return err
	}

	now := time.Now()
	feed := &feeds.Feed{
		Title:       "Warhammer Community",
		Link:        &feeds.Link{Href: "https://www.warhammer-community.com/"},
		Description: "Warhammer Community unofficial RSS feed",
		Author:      &feeds.Author{Name: "Warhammer Community"},
		Created:     now,
		Image:       &feeds.Image{Url: "https://www.warhammer-community.com/wp-content/themes/gw-community-2020/assets/images/apple-touch-icon.png"},
	}

	doc.Find(".post-item").Each(func(i int, s *goquery.Selection) {
		imageStr, _ := s.Find(".post-item__img-container").Attr("style")

		re := regexp.MustCompile(`background-image: url\('(.*)'\);`)
		image := re.FindStringSubmatch(imageStr)[1]

		title := s.Find(".post-item__title ").Text()
		excerpt := s.Find(".post-feed__excerpt").Text()
		link, _ := s.Attr("href")
		dateStr := s.Find(".post-item__date").Text()
		date, _ := time.Parse("2 Jan 06", strings.TrimSpace(dateStr))

		feed.Add(&feeds.Item{
			Title:       title,
			Link:        &feeds.Link{Href: link},
			Description: "<img style='width: 100%;' src='" + image + "'><br/><br/>" + excerpt,
			Author:      &feeds.Author{Name: "Warhammer Community"},
			Created:     date,
			Enclosure:   &feeds.Enclosure{Url: image, Length: EnclosureSize(image), Type: "image/jpg"},
		})
	})

	atom, err := feed.ToAtom()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile("./static/warhammer-community.atom", []byte(atom), 0644)
	if err != nil {
		return err
	}

	log.Println("warhammer-community.atom written")

	return err
}

func ChromeScrape(url string, selector string, f func(context.Context) error) {
	log.Printf("Executig Chrome scrape on %s (checking %s)", url, selector)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.Flag("headless", true),
		chromedp.Flag("hide-scrollbars", false),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.UserAgent(UserAgent),
	)

	catx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(catx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(selector),
		chromedp.ActionFunc(f),
	)

	if err != nil {
		log.Fatal(err)
	}
}

func ScrapeWarhammerCommunity() {
	ChromeScrape(`https://www.warhammer-community.com/latest-news-features/`, `#articles`, WarhammerCommunityProcessing)
}

func main() {
	port := flag.String("p", "8100", "port to serve on")
	flag.Parse()

	log.Println("Performing initial scrape")
	ScrapeWarhammerCommunity()

	log.Println("Setting up cron")
	c := cron.New()
	c.AddFunc("@every 1h", func() { ScrapeWarhammerCommunity() })
	c.Start()

	log.Println(c.Entries())

	http.Handle("/", http.FileServer(http.Dir("./static")))
	log.Printf("Booting static file server on port: %s\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
