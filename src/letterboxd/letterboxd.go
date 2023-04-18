package letterboxd

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app           plugintypes.App
	parameterName string // Syndication parameter
	blogURL       string // Blog URL
	section       string // Watches section
	username      string // Letterboxd Username
	token         string // Micropub Token
}

// Letterboxd RSS Feed Struct
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

// Feed Channel Struct
type Channel struct {
	Title       string    `xml:"title"`
	Description string    `xml:"description"`
	Link        string    `xml:"link"`
	Items       []RSSItem `xml:"item"`
}

// Feed Item data Struct
type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	WatchedDate string `xml:"https://letterboxd.com watchedDate"`
	Rewatch     string `xml:"https://letterboxd.com rewatch"`
	FilmArt     string
}

func GetPlugin() (plugintypes.SetConfig, plugintypes.SetApp, plugintypes.UI2) {
	p := &plugin{}
	return p, p, p
}

func (p *plugin) SetConfig(config map[string]any) {
	p.parameterName = "syndication" // default

	for key, value := range config {
		switch key {
		case "section":
			p.section = value.(string)
		case "username":
			p.username = value.(string)
		case "blogurl":
			p.blogURL = value.(string)
		case "token":
			p.token = value.(string)
		default:
			fmt.Println("Unknown config key:", key)
		}
	}
}

func (p *plugin) RenderWithDocument(rc plugintypes.RenderContext, doc *goquery.Document) {
	post, err := p.app.GetPost(rc.GetPath())
	if err != nil || post == nil {
		return
	}
	letterboxdLink, ok := post.GetParameters()[p.parameterName]
	if !ok || len(letterboxdLink) == 0 {
		return
	}
	if err != nil {
		fmt.Println("Letterboxd plugin: " + err.Error())
		return
	}
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	for _, link := range letterboxdLink {
		boxd := "boxd"
		if strings.Contains(link, boxd) {
			doc.Find("main.h-entry article div.e-content p img").AddClass("u-photo").SetAttr("alt", "Film Poster") // Add Microformat and Alt attr to Film Poster
		} else {
			break
		}
	}
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app

	ticker := time.NewTicker(1 * time.Hour) // Check every hour
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				p.fetchWatches()
			}
		}
	}()
}

func (p *plugin) fetchWatches() {
	// Fetch Letterboxd RSS feed
	resp, err := http.Get("https://letterboxd.com/" + p.username + "/rss")
	if err != nil {
		fmt.Println("Error fetching Letterboxd feed:", err)
		return
	}
	defer resp.Body.Close()

	// Parse Letterboxd feed
	rss := RSS{}
	err = xml.NewDecoder(resp.Body).Decode(&rss)
	if err != nil {
		fmt.Println("Error parsing Letterboxd feed:", err)
		return
	}

	// Find the last item in Letterboxd feed
	lastItem := rss.Channel.Items[:1]
	for _, item := range lastItem {
		// Extract fields from Letterboxd feed item
		title := item.Title
		link := item.Link
		watchedDate := item.WatchedDate
		rewatch := item.Rewatch
		watchStatus := ""

		if rewatch == "Yes" {
			watchStatus = "Rewatched "
		}

		// Extract Film poster from the Description field
		re := regexp.MustCompile(`<img.*?src="(.*?)".*?>`)
		matches := re.FindStringSubmatch(item.Description)
		filmArt := ""
		if len(matches) > 1 {
			filmArt = matches[1]
		}
		// Set Description and Slug values
		desc := item.Description
		watchName := strings.Replace(strings.TrimPrefix(link, "https://letterboxd.com/"+p.username+"/film/"), "/", "", -1)
		slug := fmt.Sprintf("%s-%s", watchedDate, watchName)

		// Fetch Watches section feed and extract last Watch permalink
		resp, err := http.Get(p.blogURL + "/" + p.section)
		if err != nil {
			fmt.Println("Error fetching HTML page:", err)
			return
		}
		defer resp.Body.Close()
		htmlDoc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			fmt.Println("Error parsing HTML page:", err)
			return
		}
		uURL := ""
		htmlDoc.Find(".h-entry a.u-url").First().Each(func(i int, s *goquery.Selection) {
			uURL = s.AttrOr("href", "")
		})
		if uURL == "" {
			fmt.Println("Error: u-url not found")
			return
		}
		ownWatch := p.blogURL + uURL
		resp, err = http.Get(ownWatch)
		if err != nil {
			fmt.Println("Error fetching u-url HTML page:", err)
			return
		}
		defer resp.Body.Close()
		htmlDoc, err = goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			fmt.Println("Error parsing u-url HTML page:", err)
			return
		}
		syndicated := ""
		htmlDoc.Find(".letterboxd").Each(func(i int, s *goquery.Selection) {
			syndicated = s.AttrOr("href", "")
		})

		// Update Watches with new entry
		if syndicated != link {
			// Send data via HTTP POST / Micropub
			formData := url.Values{
				"section":     {p.section},
				"slug":        {slug},
				"syndication": {link},
				"content":     {"<p class=\"p-name\">🍿 " + watchStatus + "<b>" + title + "</b></p>" + desc},
				// "filmArt":     {filmArt}, // Reference only(?)
			}

			// Create the Micropub request
			req, err := http.NewRequest("POST", p.blogURL+"/micropub", strings.NewReader(formData.Encode())) // GoBlog's Micropub Endpoint
			if err != nil {
				panic(fmt.Errorf("error creating request: %v", err))
			}

			// Set the authorization header
			req.Header.Set("Authorization", "Bearer "+p.token) // Micropub Token

			// Set the content type header
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Send the HTTP POST request
			client := &http.Client{}
			res, err := client.Do(req)
			if err != nil {
				panic(fmt.Errorf("error creating request: %v", err))
			}
			defer res.Body.Close()

			// Read the response body
			resBody, err := io.ReadAll(res.Body)
			if err != nil {
				panic(fmt.Errorf("error creating request: %v", err))
			}

			// Print the response
			fmt.Println(string(resBody)) // (there's no response when sent as form?)
		} else {
			fmt.Println("Letterboxd plugin: Watches up to date at", time.Now())
		}
	}
}
