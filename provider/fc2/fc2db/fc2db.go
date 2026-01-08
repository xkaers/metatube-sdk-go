package fc2db

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	_ "time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/metatube-community/metatube-sdk-go/common/parser"
	"github.com/metatube-community/metatube-sdk-go/model"
	"github.com/metatube-community/metatube-sdk-go/provider/fc2/fc2util"
)

const ConfigKeyDatabasePath = "fc2.meta.db.path"

type Article struct {
	Code        string `gorm:"column:code;primaryKey"`
	Title       string `gorm:"column:title"`
	ReleaseDate string `gorm:"column:release_date"`
	Duration    string `gorm:"column:duration"`
	WriterID    string `gorm:"column:writer_id"`
	Tags        string `gorm:"column:tags"`
	CoverURL    string `gorm:"column:cover_url"`
	FC2URL      string `gorm:"column:fc2_url"`
}

func (Article) TableName() string {
	return "t_article"
}

type Actress struct {
	ID        string `gorm:"column:id;primaryKey"`
	Name      string `gorm:"column:name"`
	AliasName string `gorm:"column:alias_name"`
	URL       string `gorm:"column:url"`
}

func (Actress) TableName() string {
	return "t_actress"
}

type Writer struct {
	ID   string `gorm:"column:id;primaryKey"`
	Name string `gorm:"column:name"`
}

func (Writer) TableName() string {
	return "t_writer"
}

type ArticleActress struct {
	ArticleCode string `gorm:"column:article_code;primaryKey"`
	ActressID   string `gorm:"column:actress_id;primaryKey"`
}

func (ArticleActress) TableName() string {
	return "t_article_actress"
}

type Manager struct {
	db *gorm.DB
}

func New(dbPath string) (*Manager, error) {
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Return nil if file does not exist
		}
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	return &Manager{db: db}, nil
}

func (m *Manager) GetMovieInfo(id string) (*model.MovieInfo, error) {
	// Normalize ID just in case, though the caller usually does this.
	// The DB uses "code" which seems to be the numeric part or full ID?
	// Based on user sample: code="3435493", URL=".../article/3435493".
	// So it's likely the numeric part.
	numericID := fc2util.ParseNumber(id)
	if numericID == "" {
		return nil, fmt.Errorf("invalid fc2 id: %s", id)
	}

	var article Article
	if err := m.db.First(&article, "code = ?", numericID).Error; err != nil {
		return nil, err
	}

	// Get Writer (Maker)
	var writer Writer
	if article.WriterID != "" {
		m.db.First(&writer, "id = ?", article.WriterID)
	}

	// Get Actresses
	var actresses []Actress
	m.db.Table("t_actress").
		Joins("JOIN t_article_actress ON t_article_actress.actress_id = t_actress.id").
		Where("t_article_actress.article_code = ?", numericID).
		Find(&actresses)

	actorNames := make([]string, len(actresses))
	for i, actress := range actresses {
		actorNames[i] = fmt.Sprintf("%s[%s]", actress.Name, actress.ID)
	}

	// Map to MovieInfo
	info := &model.MovieInfo{
		ID:          numericID,
		Number:      fmt.Sprintf("FC2-%s", numericID),
		Title:       article.Title,
		Provider:    "FC2", // Default, caller might override
		Homepage:    article.FC2URL,
		CoverURL:    article.CoverURL,
		ThumbURL:    article.CoverURL, // Fallback
		Maker:       writer.Name,
		Actors:      actorNames,
		Runtime:     parser.ParseRuntime(article.Duration),
		ReleaseDate: parser.ParseDate(article.ReleaseDate),
		Genres:      parseTags(article.Tags),
	}

	if info.Homepage == "" {
		// Fallback homepage generation if DB is missing it
		info.Homepage = fmt.Sprintf("https://adult.contents.fc2.com/article/%s/", numericID)
	}

	return info, nil
}

func (m *Manager) GetActorInfo(id string) (*model.ActorInfo, error) {
	// Handle composite ID: Name[ID]
	if ss := regexp.MustCompile(`^.+?\[(.+)\]$`).FindStringSubmatch(id); len(ss) == 2 {
		id = ss[1]
	}

	var actress Actress
	if err := m.db.First(&actress, "id = ?", id).Error; err != nil {
		if err := m.db.First(&actress, "name LIKE ?", "%"+id+"%").Error; err != nil {
			return nil, err
		}
	}

	info := &model.ActorInfo{
		ID:       actress.ID,
		Name:     actress.Name,
		Provider: "FC2",
		Homepage: actress.URL,
		Images:   []string{}, // Add image URL if available in DB
	}
	if info.Homepage == "" {
		info.Homepage = fmt.Sprintf("https://fc2ppvdb.com/actresses/%s/", actress.ID)
	}
	if actress.AliasName != "" {
		info.Aliases = strings.Fields(actress.AliasName)
	}

	// Get movies this actress appeared in
	var articles []Article
	m.db.Table("t_article").
		Joins("JOIN t_article_actress ON t_article_actress.article_code = t_article.code").
		Where("t_article_actress.actress_id = ?", actress.ID).
		Find(&articles)

	if len(articles) > 0 {
		var lines []string
		lines = append(lines, "该演员还出演过以下作品：")
		for _, a := range articles {
			lines = append(lines, fmt.Sprintf("[FC2-%s] %s", a.Code, a.Title))
		}
		info.Summary = strings.Join(lines, "\n")
	}

	return info, nil
}

func (m *Manager) SearchActors(keyword string) (results []*model.ActorSearchResult, err error) {
	// Handle composite ID: Name[ID]
	if ss := regexp.MustCompile(`^.+?\[(.+)\]$`).FindStringSubmatch(keyword); len(ss) == 2 {
		keyword = ss[1]
	}

	var actresses []Actress
	if err := m.db.Where("id = ?", keyword).Find(&actresses).Error; err != nil || len(actresses) == 0 {
		if err := m.db.Where("name LIKE ?", "%"+keyword+"%").Find(&actresses).Error; err != nil {
			return nil, err
		}
	}

	for _, actress := range actresses {
		result := &model.ActorSearchResult{
			ID:       actress.ID,
			Name:     actress.Name,
			Provider: "FC2",
			Homepage: actress.URL,
			Images:   []string{},
		}
		if result.Homepage == "" {
			result.Homepage = fmt.Sprintf("https://fc2ppvdb.com/actresses/%s/", actress.ID)
		}
		if actress.AliasName != "" {
			result.Aliases = strings.Fields(actress.AliasName)
		}
		results = append(results, result)
	}
	return
}

func parseTags(tags string) []string {
	if tags == "" {
		return nil
	}
	// User sample: "人妻,ハメ撮り,素人..." (comma separated)
	// But sometimes might be spaces? Assuming commas based on sample.
	parts := strings.Split(tags, ",")
	var result []string
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			result = append(result, s)
		}
	}
	return result
}
