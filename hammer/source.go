package hammer

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"github.com/Sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"path"
)

var (
	ErrBadResponse = errors.New("bad response")
	ErrBadHash     = errors.New("bad hash")
	ErrBadHashType = errors.New("bad hash type")
)

type Resource struct {
	URL      string `yaml:"url"`
	HashType string `yaml:"hash-type"`
	Hash     string `yaml:"hash"`
	Unpack   bool   `yaml:"unpack"`
}

func (s *Resource) Name() string {
	_, name := path.Split(s.URL)
	return name
}

func (s *Resource) Download() ([]byte, error) {
	logger := logrus.WithField("name", s.Name())
	logger.Info("getting resource")

	client := http.Client{} // TODO: caching of some kind?
	resp, err := client.Get(s.URL)
	if err != nil {
		logger.WithField("error", err).Error("could not complete request")
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotModified {
		logger.WithFields(logrus.Fields{
			"code":   resp.StatusCode,
			"status": resp.Status,
		}).Error("bad response")
		return nil, ErrBadResponse
	}

	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	// checksum
	var sum string
	switch s.HashType {
	case "md5":
		summer := md5.New()
		summer.Write(body)
		sum = hex.EncodeToString(summer.Sum(nil))
	case "sha1":
		summer := sha1.New()
		summer.Write(body)
		sum = hex.EncodeToString(summer.Sum(nil))
	default:
		logger.WithField("type", s.HashType).Error("bad hash type (try md5 or sha1)")
		return body, ErrBadHashType
	}

	if sum != s.Hash {
		logger.WithFields(logrus.Fields{
			"provided": s.Hash,
			"actual":   sum,
		}).Error("actual hash did not match provided hash")
		return body, ErrBadHash
	}

	return body, err
}
