package cache

import (
	"io/ioutil"
	"os"
	"path"
)

// FSCache is an implementation of Cache that stores information in the
// filesystem
type FSCache struct {
	Root string
}

// NewFSCache returns a FileSystemcache with defaults set, and makes
// sure that the FSCache has a directory to store info in.
func NewFSCache(root string) (Cache, error) {
	fs := &FSCache{root}
	err := fs.Setup()

	return fs, err
}

// Setup creates the directory for storage, if it doesn't exist
func (fs *FSCache) Setup() error {
	err := os.MkdirAll(fs.Root, os.ModeDir|0777)
	return err
}

// Get returns the content under key or ErrNoSuchKey
func (fs *FSCache) Get(key string) ([]byte, error) {
	content, err := ioutil.ReadFile(path.Join(fs.Root, key))
	if os.IsNotExist(err) {
		return content, ErrNoSuchKey
	}

	return content, err
}

// Set writes the content under the given key
func (fs *FSCache) Set(key string, content []byte) error {
	return ioutil.WriteFile(
		path.Join(fs.Root, key),
		content,
		0600,
	)
}

// Keys returns the keys managed by this cache
func (fs *FSCache) Keys() ([]string, error) {
	keys := []string{}
	files, err := ioutil.ReadDir(fs.Root)
	if err != nil {
		return keys, err
	}

	for _, entry := range files {
		if entry.IsDir() {
			continue
		}

		keys = append(keys, entry.Name())
	}

	return keys, nil
}

// Delete removes a key or returns ErrNoSuchKey
func (fs *FSCache) Delete(key string) error {
	err := os.Remove(path.Join(fs.Root, key))
	if os.IsNotExist(err) {
		return ErrNoSuchKey
	}
	return err
}
