package fc2ppvdb

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/gocolly/colly/v2"
	"golang.org/x/text/language"

	"github.com/metatube-community/metatube-sdk-go/common/parser"
	"github.com/metatube-community/metatube-sdk-go/model"
	"github.com/metatube-community/metatube-sdk-go/provider"
	"github.com/metatube-community/metatube-sdk-go/provider/fc2/fc2util"
	"github.com/metatube-community/metatube-sdk-go/provider/internal/scraper"
)

var (
	_ provider.MovieProvider = (*FC2PPVDBMovie)(nil)
	_ provider.MovieSearcher = (*FC2PPVDBMovie)(nil)
)

const (
	FC2PPVDBMovieName = "FC2PPVDBMovie"
)

const (
	movieBaseURL   = "https://fc2ppvdb.com/"
	movieURL       = "https://fc2ppvdb.com/articles/%s"
	searchMovieURL = "https://fc2ppvdb.com/search?stype=title&keyword=%s"
)

type FC2PPVDBMovie struct {
	*scraper.Scraper
}

func FC2PPVDBMovieNew() *FC2PPVDBMovie {
	return &FC2PPVDBMovie{scraper.NewDefaultScraper(FC2PPVDBMovieName, movieBaseURL, Priority, language.Japanese)}
}

func (fc2ppvdbMovie *FC2PPVDBMovie) NormalizeMovieID(id string) string {
	return fc2util.ParseNumber(id)
}

func (fc2ppvdbMovie *FC2PPVDBMovie) GetMovieInfoByID(id string) (info *model.MovieInfo, err error) {
	return fc2ppvdbMovie.GetMovieInfoByURL(fmt.Sprintf(movieURL, id))
}

func (fc2ppvdbMovie *FC2PPVDBMovie) ParseMovieIDFromURL(rawURL string) (string, error) {
	homepage, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return path.Base(homepage.Path), nil
}

func (fc2ppvdbMovie *FC2PPVDBMovie) GetMovieInfoByURL(rawURL string) (info *model.MovieInfo, err error) {
	id, err := fc2ppvdbMovie.ParseMovieIDFromURL(rawURL)
	if err != nil {
		return
	}

	info = &model.MovieInfo{
		ID:            id,
		Number:        fmt.Sprintf("FC2-%s", id),
		Provider:      fc2ppvdbMovie.Name(),
		Homepage:      rawURL,
		Actors:        []string{},
		PreviewImages: []string{},
		Genres:        []string{},
	}

	c := fc2ppvdbMovie.ClonedCollector()

	// 标题
	c.OnXML(`//div[@class="container lg:px-5 px-2 py-12 mx-auto"]//h2/a`, func(e *colly.XMLElement) {
		info.Title = strings.TrimSpace(e.Text)
	})

	// 分类标签
	c.OnXML(`//div[contains(text(),"タグ：")]/span/a[starts-with(@href,"/tags/")]`,
		func(e *colly.XMLElement) {
			info.Genres = append(info.Genres, strings.TrimSpace(e.Text))
		})

	// 女优
	c.OnXML(`//div[contains(text(),"女優：")]/span/a[starts-with(@href,"/actresses/")]`,
		func(e *colly.XMLElement) {
			// 获取演员名字
			name := strings.TrimSpace(e.Text)

			// 从href中提取ID
			href := e.Attr("href")
			parts := strings.Split(strings.TrimSuffix(href, "/"), "/")
			id := ""
			if len(parts) > 0 {
				id = parts[len(parts)-1]
			}

			// 组合成"名字(ID)"格式
			if id != "" {
				info.Actors = append(info.Actors, fmt.Sprintf("%s^%s", name, id))
			} else {
				info.Actors = append(info.Actors, name)
			}
		})

	// 时长
	c.OnXML(`//div[contains(text(),"収録時間：")]/span`,
		func(e *colly.XMLElement) {
			info.Runtime = parser.ParseRuntime(e.Text)
		})

	// 发布日期
	c.OnXML(`//div[contains(text(),"販売日：")]/span`,
		func(e *colly.XMLElement) {
			info.ReleaseDate = parser.ParseDate(e.Text)
		})

	// 作者
	c.OnXML(`//div[contains(text(),"販売者：")]/span/a`,
		func(e *colly.XMLElement) {
			info.Maker = strings.TrimSpace(e.Text)
		})

	// 是否有码
	c.OnXML(`//div[contains(text(),"モザイク：")]/span`,
		func(e *colly.XMLElement) {
			uncensoredStr := strings.TrimSpace(e.Text)
			if uncensoredStr == "無" {
				info.Label = "无码"
			} else if uncensoredStr == "有" {
				info.Label = "有码"
			}
		})

	// 根据喜欢, 点赞, 点踩人数来计算评分
	var like, upvote, downvote int
	// 点赞
	c.OnXML(`//div[@id='unheart']/span[contains(@class, 'text-white')]`,
		func(e *colly.XMLElement) {
			like = parser.ParseInt(strings.TrimSpace(e.Text))
		})

	// 好评
	c.OnXML(`//div[@id='up-count']/span[@id='up-counter']`,
		func(e *colly.XMLElement) {
			upvote = parser.ParseInt(strings.TrimSpace(e.Text))
		})

	// 差评
	c.OnXML(`//div[@id='down-count']/span[@id='down-counter']`,
		func(e *colly.XMLElement) {
			downvote = parser.ParseInt(strings.TrimSpace(e.Text))
		})

	info.Score = calculateScore(like, upvote, downvote)
	// 预览视频
	c.OnXML(`//a[contains(text(),"サンプル動画")]`,
		func(e *colly.XMLElement) {
			info.PreviewVideoURL = e.Attr("href")
		})

	// 封面图
	c.OnXML(fmt.Sprintf(`//img[@alt="%s"]`, id),
		func(e *colly.XMLElement) {
			info.CoverURL = e.Attr("src")
		})

	// Cover (fallbacks)
	c.OnScraped(func(_ *colly.Response) {
		if info.CoverURL != "" {
			info.ThumbURL = info.CoverURL
		}
	})
	if vErr := c.Visit(info.Homepage); vErr != nil {
		err = vErr
	}
	return
}

func (fc2ppvdbMovie *FC2PPVDBMovie) NormalizeMovieKeyword(keyword string) string {
	return fc2util.ParseNumber(keyword)
}

func (fc2ppvdbMovie *FC2PPVDBMovie) SearchMovie(keyword string) (results []*model.MovieSearchResult, err error) {
	c := fc2ppvdbMovie.ClonedCollector()
	c.ParseHTTPErrorResponse = true
	c.SetRedirectHandler(func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	})

	c.OnResponse(func(r *colly.Response) {
		var loc *url.URL
		if loc, err = url.Parse(r.Request.AbsoluteURL(r.Headers.Get("Location"))); err != nil {
			return
		}
		if regexp.MustCompile(`/articles/\d+`).MatchString(loc.Path) {
			var info *model.MovieInfo
			if info, err = fc2ppvdbMovie.GetMovieInfoByURL(loc.String()); err != nil {
				return
			}
			results = append(results, info.ToSearchResult())
		}
	})

	err = c.Visit(fmt.Sprintf(searchMovieURL, url.QueryEscape(keyword)))
	return
}

// 计算0-5分制评分
func calculateScore(like, upvote, downvote int) float64 {
	// 定义权重和先验值
	wLike := 1.0
	wUp := 1.5
	wDown := 1.5
	priorNumerator := 1.0
	priorDenominator := 2.0

	// 计算基础评分 (0~1)
	numerator := wLike*float64(like) + wUp*float64(upvote) + priorNumerator
	denominator := wLike*float64(like) + wUp*float64(upvote) + wDown*float64(downvote) + priorDenominator

	rawScore := numerator / denominator

	// 映射到0-5分制
	score := 5.0 * rawScore

	// 保留1位小数（四舍五入）
	return math.Round(score*10) / 10
}
