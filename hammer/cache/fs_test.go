package cache

import (
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

type FSCacheSuite struct {
	suite.Suite
	cache *FSCache
	tmp   string
}

func (fs *FSCacheSuite) SetupTest() {
	tmp, err := ioutil.TempDir("", "hammer-cache-test")
	if err != nil {
		panic(err)
	}
	fs.tmp = tmp

	cache, err := NewFSCache(tmp)
	if err != nil {
		panic(err)
	}
	fs.cache = cache.(*FSCache)
}

func (fs *FSCacheSuite) TeardownTest() {
	err := os.RemoveAll(fs.tmp)
	if err != nil {
		panic(err)
	}
}

func (fs *FSCacheSuite) TestGet() {
	err := ioutil.WriteFile(path.Join(fs.tmp, "test"), []byte("test"), 0600)
	fs.Assert().Nil(err)

	content, err := fs.cache.Get("test")
	fs.Assert().Nil(err)
	fs.Assert().Equal(content, []byte("test"))
}

func (fs *FSCacheSuite) TestGetNotFound() {
	content, err := fs.cache.Get("test")
	fs.Assert().Equal(err, ErrNoSuchKey)
	fs.Assert().Equal(len(content), 0)
}

func (fs *FSCacheSuite) TestSet() {
	err := fs.cache.Set("test", []byte("test"))
	fs.Assert().Nil(err)

	content, err := ioutil.ReadFile(path.Join(fs.tmp, "test"))
	fs.Assert().Nil(err)
	fs.Assert().Equal(content, []byte("test"))
}

func (fs *FSCacheSuite) TestKeys() {
	err := fs.cache.Set("test", []byte("test"))
	fs.Assert().Nil(err)

	keys, err := fs.cache.Keys()
	fs.Assert().Nil(err)
	fs.Assert().Equal(keys, []string{"test"})
}

func (fs *FSCacheSuite) TestDelete() {
	err := ioutil.WriteFile(path.Join(fs.tmp, "test"), []byte("test"), 0600)
	fs.Assert().Nil(err)

	err = fs.cache.Delete("test")
	fs.Assert().Nil(err)
}

func (fs *FSCacheSuite) TestDeleteNotFound() {
	err := fs.cache.Delete("test")
	fs.Assert().Equal(err, ErrNoSuchKey)
}

func TestFSCacheSuite(t *testing.T) {
	suite.Run(t, new(FSCacheSuite))
}
