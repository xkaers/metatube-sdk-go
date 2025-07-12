package fc2ppvdb

import (
	"github.com/metatube-community/metatube-sdk-go/provider"
)

const Priority = 1000 + 2

func init() {
	provider.Register(FC2PPVDBMovieName, FC2PPVDBMovieNew)
	provider.Register(FC2PPVDBActorName, FC2PPVDBActorNew)
}
