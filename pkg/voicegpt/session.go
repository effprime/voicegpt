package voicegpt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/effprime/voicegpt/pkg/gptclient"
)

type SessionStorage interface {
	Get(ctx context.Context, id string) (*Session, error)
	Save(ctx context.Context, s *Session) error
}

type Session struct {
	ID       string
	Messages []gptclient.Message `json:"messages"`
}

type FileSessionStorage struct {
	mutex     sync.Mutex
	directory string // Directory where session files are stored
}

func NewFileSessionStorage(dir string) (*FileSessionStorage, error) {
	_, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		err = os.Mkdir(dir, 0777)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return &FileSessionStorage{directory: dir}, nil
}

func (fss *FileSessionStorage) Save(ctx context.Context, s *Session) error {
	fss.mutex.Lock()
	defer fss.mutex.Unlock()

	// Serialize the session object
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	// Create a file name based on session ID
	filename := fmt.Sprintf("%s.json", s.ID)
	filePath := filepath.Join(fss.directory, filename)

	file := &os.File{}
	_, err = os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		file, err = os.Create(filePath)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		file, err = os.OpenFile(filePath, os.O_WRONLY, 0666)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	// Write data to file
	_, err = file.Write(data)
	return err
}

func (fss *FileSessionStorage) Get(ctx context.Context, id string) (*Session, error) {
	fss.mutex.Lock()
	defer fss.mutex.Unlock()

	if id == "" {
		return nil, nil
	}

	// Construct file path
	filename := fmt.Sprintf("%s.json", id)
	filePath := filepath.Join(fss.directory, filename)

	_, err := os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Deserialize data into a session object
	var session Session
	err = json.Unmarshal(data, &session)
	if err != nil {
		return nil, err
	}

	return &session, nil
}
