package fc2ppvdb

import (
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/metatube-community/metatube-sdk-go/common/fetch"
	"golang.org/x/text/language"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/metatube-community/metatube-sdk-go/model"
	"github.com/metatube-community/metatube-sdk-go/provider"
	_ "github.com/metatube-community/metatube-sdk-go/provider/fc2/fc2util"
	"github.com/metatube-community/metatube-sdk-go/provider/internal/scraper"
)

var (
	_ provider.ActorProvider = (*FC2PPVDBActor)(nil)
	_ provider.ActorSearcher = (*FC2PPVDBActor)(nil)
	_ provider.Fetcher       = (*FC2PPVDBActor)(nil)
)

const (
	FC2PPVDBActorName = "FC2PPVDBActor"
)

const (
	actorBaseURL   = "https://fc2ppvdb.com/"
	actorURL       = "https://fc2ppvdb.com/actresses/%s"
	searchActorURL = "https://fc2ppvdb.com/search?stype=actress&keyword=%s"
)

type FC2PPVDBActor struct {
	*fetch.Fetcher
	*scraper.Scraper
}

func FC2PPVDBActorNew() *FC2PPVDBActor {
	return &FC2PPVDBActor{
		Fetcher: fetch.Default(&fetch.Config{SkipVerify: false, Timeout: time.Second * 30, UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36 Edg/138.0.0.0"}),
		Scraper: scraper.NewDefaultScraper(FC2PPVDBActorName, actorBaseURL, Priority, language.Japanese),
	}

}

func (fc2ppvdbActor *FC2PPVDBActor) GetActorInfoByID(id string) (info *model.ActorInfo, err error) {
	return fc2ppvdbActor.GetActorInfoByURL(fmt.Sprintf(actorURL, id))
}

func (fc2ppvdbActor *FC2PPVDBActor) ParseActorIDFromURL(rawURL string) (id string, err error) {
	homepage, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return path.Base(homepage.Path), nil
}

func (fc2ppvdbActor *FC2PPVDBActor) GetActorInfoByURL(rawURL string) (info *model.ActorInfo, err error) {
	id, err := fc2ppvdbActor.ParseActorIDFromURL(rawURL)
	if err != nil {
		return
	}

	info = &model.ActorInfo{
		ID:       id,
		Provider: fc2ppvdbActor.Name(),
		Homepage: rawURL,
		Aliases:  []string{},
		Images:   []string{},
	}

	c := fc2ppvdbActor.ClonedCollector()

	// Name
	c.OnXML(
		`
        //div[contains(@class, 'sm:w-11/12') and 
            contains(@class, 'px-2') and 
            contains(@class, 'text-white') and 
            contains(@class, 'title-font') and 
            contains(@class, 'text-lg') and 
            contains(@class, 'font-medium')]/text()[1]`,
		func(e *colly.XMLElement) {
			info.Name = strings.TrimSpace(e.Text)
		})

	// Aliases
	c.OnXML(`//*[@id="aliases"]`, func(e *colly.XMLElement) {
		for _, alias := range strings.Split(e.Text, " ") {
			info.Aliases = append(info.Aliases, strings.TrimSpace(alias))
		}
	})

	// Image (profile)
	c.OnXML(`//div[contains(@class, 'h-24') and contains(@class, 'w-24') and contains(@class, 'overflow-hidden')]/img/@src`, func(e *colly.XMLElement) {
		info.Images = append(info.Images, strings.TrimSpace(e.Text))
	})

	// Image (fallback) lazyload
	c.OnXML(
		`//div[contains(@class, 'h-24') and contains(@class, 'w-24') and contains(@class, 'overflow-hidden')]//img[contains(@class, 'lazyload')]/@data-src`,
		func(e *colly.XMLElement) {
			if len(info.Images) == 0 {
				info.Images = append(info.Images, strings.TrimSpace(e.Text))
			}
		},
	)

	err = c.Visit(info.Homepage)
	return
}

func (fc2ppvdbActor *FC2PPVDBActor) SearchActor(keyword string) (results []*model.ActorSearchResult, err error) {
	c := fc2ppvdbActor.ClonedCollector()
	c.ParseHTTPErrorResponse = true
	c.SetRedirectHandler(func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	})

	c.OnResponse(func(r *colly.Response) {
		var loc *url.URL
		if loc, err = url.Parse(r.Request.AbsoluteURL(r.Headers.Get("Location"))); err != nil {
			return
		}
		if regexp.MustCompile(`/actresses/\d+`).MatchString(loc.Path) {
			var info *model.ActorInfo
			if info, err = fc2ppvdbActor.GetActorInfoByURL(loc.String()); err != nil {
				return
			}
			results = append(results, info.ToSearchResult())
		}
	})

	parts := strings.Split(keyword, "^")
	actorId := ""
	if len(parts) > 1 {
		actorId = parts[len(parts)-1]
	}
	_, err = strconv.Atoi(actorId)
	if err == nil {
		err = c.Visit(fmt.Sprintf(actorURL, url.QueryEscape(actorId)))
	} else {
		err = c.Visit(fmt.Sprintf(searchActorURL, url.QueryEscape(keyword)))
	}
	return
}
