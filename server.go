package main

import (
	"time"
  "log"
	"strings"
	"context"
	"regexp"
	"io/ioutil"
	"flag"
	"net/http"
	"github.com/gorilla/feeds"
  "github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
	"github.com/robfig/cron/v3"
)

func ChromeScrape() {
	log.Println("Executig Chrome scrape")
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.93 Safari/537.36"

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.Flag("headless", true),
		chromedp.Flag("hide-scrollbars", false),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.UserAgent(userAgent),
	)

	catx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(catx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()


	// run task list
	err := chromedp.Run(ctx,
		chromedp.Navigate(`https://www.warhammer-community.com/latest-news-features/"`),
		chromedp.WaitVisible(`#articles`),
		chromedp.ActionFunc(func(ctx context.Context) error {
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
				log.Fatal(err)
			}

			now := time.Now()
			feed := &feeds.Feed{
				Title:       "Warhammer Community",
				Link:        &feeds.Link{Href: "https://www.warhammer-community.com/"},
				Description: "Warhammer Community unofficial RSS feed",
				Author:      &feeds.Author{Name: "Warhammer Community"},
				Created:     now,
				Image: &feeds.Image{Url: "https://www.warhammer-community.com/wp-content/themes/gw-community-2020/assets/images/apple-touch-icon.png"},
			}

			// Find the review items
			doc.Find(".post-item").Each(func(i int, s *goquery.Selection) {
				// For each item found, get the band and title
				imageStr, _ := s.Find(".post-item__img-container").Attr("style")

				re := regexp.MustCompile(`background-image: url\('(.*)'\);`)
				image := re.FindStringSubmatch(imageStr)[1]

				title := s.Find(".post-item__title ").Text()
				excerpt := s.Find(".post-feed__excerpt").Text()
				link, _ := s.Attr("href")
				dateStr := s.Find(".post-item__date").Text()
				date, _ := time.Parse("2 Jan 06", strings.TrimSpace(dateStr))

				feed.Items = append(feed.Items, &feeds.Item{
            Title:       title,
            Link:        &feeds.Link{Href: link},
            Description: "<img src='" +  image + "'><br/><br/>" + excerpt,
            Author:      &feeds.Author{Name: "Warhammer Community"},
						Created:     date,
						// TODO: Proper length
						Enclosure:   &feeds.Enclosure{Url: image, Length: "123456", Type: "image/jpg"},
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
		}),
	)

	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	port := flag.String("p", "8100", "port to serve on")
	flag.Parse()

	log.Println("Performing initial scrape")
	ChromeScrape()

	log.Println("Setting up cron")
	c := cron.New()
	c.AddFunc("@every 1h", func() { ChromeScrape() })
	c.Start()


	log.Println(c.Entries())

	http.Handle("/", http.FileServer(http.Dir("./static")))
	log.Printf("Booting static file server on port: %s\n", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
