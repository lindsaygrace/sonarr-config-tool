package utils

import (
	"emperror.dev/errors"
	"fmt"
	"github.com/clbanning/mxj/v2"
	"github.com/lindsaygrace/go-sonarr-client"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

// GetSeriesID retrieves a tvdb id from a media folder, either via a nfo file if present, or via sonarr's search api
func GetSeriesID(dir *os.File, client *sonarr.Sonarr, logger Logger) (int, error) {
	var id int
	var nfo = filepath.Join(dir.Name(), "tvshow.nfo")

	if FileExists(nfo) {
		file, err := os.Open(nfo)
		if err != nil {
			return 0, err
		}
		logger.Debug(fmt.Sprintf("found nfo file %s", nfo))

		contents, err := ioutil.ReadAll(file)
		if err != nil {
			return 0, err
		}

		mv, err := mxj.NewMapXml(contents) // unmarshal
		if err != nil {
			return 0, err
		}

		x, err := mv.ValueForPath("tvshow.id")
		if err != nil {
			return 0, err
		}

		id, err = strconv.Atoi(x.(string))
		if err != nil {
			return 0, err
		}

	} else {
		fileInfo, err := dir.Stat()
		if err != nil {
			return 0, err
		}
		results, err := client.Search(fileInfo.Name())
		if err != nil {
			return 0, err
		}

		switch x := len(results); x {
		case 1:
			id = results[0].TvdbID
		case 0:
			return 0, errors.New("No results found")
		default:
			return 0, errors.New("Multiple results found")
		}

	}

	logger.Debug(fmt.Sprintf("found id %v", id))
	return id, nil
}
