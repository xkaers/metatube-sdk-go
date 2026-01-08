package fc2ppvdb

import (
	"fmt"
	"net/url"
	"path"

	"golang.org/x/text/language"

	"github.com/metatube-community/metatube-sdk-go/model"
	"github.com/metatube-community/metatube-sdk-go/provider"
	"github.com/metatube-community/metatube-sdk-go/provider/fc2/fc2db"
	"github.com/metatube-community/metatube-sdk-go/provider/fc2/fc2util"
	"github.com/metatube-community/metatube-sdk-go/provider/internal/scraper"
)

var (
	_ provider.MovieProvider = (*FC2PPVDB)(nil)
	_ provider.ConfigSetter  = (*FC2PPVDB)(nil)
)

const (
	Name     = "FC2PPVDB"
	Priority = 1000 - 2
)

const (
	baseURL  = "https://fc2ppvdb.com/"
	movieURL = "https://fc2ppvdb.com/articles/%s"
)

type FC2PPVDB struct {
	*scraper.Scraper
	db *fc2db.Manager
}

func New() *FC2PPVDB {
	return &FC2PPVDB{Scraper: scraper.NewDefaultScraper(Name, baseURL, Priority, language.Japanese)}
}

func (fc2ppvdb *FC2PPVDB) NormalizeMovieID(id string) string {
	return fc2util.ParseNumber(id)
}

func (fc2ppvdb *FC2PPVDB) SetConfig(c provider.Config) error {
	if c.Has(fc2db.ConfigKeyDatabasePath) {
		dbPath, _ := c.GetString(fc2db.ConfigKeyDatabasePath)
		db, err := fc2db.New(dbPath)
		if err != nil {
			return err
		}
		fc2ppvdb.db = db
	}
	return nil
}

func (fc2ppvdb *FC2PPVDB) GetMovieInfoByID(id string) (info *model.MovieInfo, err error) {
	return fc2ppvdb.GetMovieInfoByURL(fmt.Sprintf(movieURL, id))
}

func (fc2ppvdb *FC2PPVDB) ParseMovieIDFromURL(rawURL string) (string, error) {
	homepage, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return path.Base(homepage.Path), nil
}

func (fc2ppvdb *FC2PPVDB) GetMovieInfoByURL(rawURL string) (info *model.MovieInfo, err error) {
	id, err := fc2ppvdb.ParseMovieIDFromURL(rawURL)
	if err != nil {
		return
	}

	if fc2ppvdb.db == nil {
		// If no DB is configured, this provider is now dead/invalid as per user request,
		// or we could return an error saying DB is required.
		// Retaining old behavior (scraping) is explicitly not requested ("remove implementation").
		// So we return an error.
		return nil, provider.ErrProviderNotFound // Or a more specific error?
	}

	// Internal query helper that uses just the ID
	info, err = fc2ppvdb.db.GetMovieInfo(id)
	if err != nil {
		return nil, err
	}

	// Ensure provider and homepage are set correctly
	info.Provider = fc2ppvdb.Name()
	info.Homepage = rawURL
	return
}

func init() {
	provider.Register(Name, New)
}
