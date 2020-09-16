package app

import (
	"fmt"
	"io/ioutil"

	"github.com/gardener/docode/pkg/api"
)

// Manifest creates documentation model from configration file
func Manifest(filePath string) *api.Documentation {
	var (
		docs *api.Documentation
	)
	configBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(fmt.Sprintf("%v\n", err))
	}
	if docs, err = api.Parse(configBytes); err != nil {
		panic(fmt.Sprintf("%v\n", err))
	}
	return docs
}
