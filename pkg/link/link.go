package link

import "net/url"

func Build(elem ...string) (string, error) {
	if len(elem) == 0 {
		return "", nil
	}
	return url.JoinPath(elem[0], elem[1:]...)
}

func MustBuild(elem ...string) string {
	res, err := Build(elem...)
	if err != nil {
		panic(err)
	}
	return res
}
