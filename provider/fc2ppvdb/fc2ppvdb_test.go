package fc2ppvdb

import (
	"testing"

	"github.com/metatube-community/metatube-sdk-go/provider/internal/testkit"
)

func TestFC2PPVDBMovie_GetMovieInfoByID(t *testing.T) {
	testkit.Test(t, FC2PPVDBMovieNew, []string{
		"2812904",
		"4669533",
	})
}
