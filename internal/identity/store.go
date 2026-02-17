package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

var (
	ErrNotFound  = errors.New("identity not found")
	ErrDuplicate = errors.New("identity already exists")
	ErrDecrypt   = errors.New("failed to decrypt identity store (wrong password?)")
)

type storeFile struct {
	Salt []byte `json:"salt"`
	Data []byte `json:"data"`
}

// FileStore implements Provider with AES-256-GCM encrypted file persistence.
type FileStore struct {
	mu         sync.RWMutex
	path       string
	key        []byte
	salt       []byte
	identities map[string]Identity
}

// NewFileStore opens or creates an encrypted identity store at the given path.
// If the file does not exist, a new store is created with a fresh salt.
// If the file exists, it is decrypted using the provided password.
func NewFileStore(path string, password []byte) (*FileStore, error) {
	s := &FileStore{
		path:       path,
		identities: make(map[string]Identity),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			salt, err := GenerateSalt()
			if err != nil {
				return nil, err
			}
			s.salt = salt
			s.key = DeriveKey(password, salt)
			return s, s.save()
		}
		return nil, err
	}

	var sf storeFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("corrupt identity store: %w", err)
	}

	s.salt = sf.Salt
	s.key = DeriveKey(password, sf.Salt)

	plaintext, err := Decrypt(s.key, sf.Data)
	if err != nil {
		return nil, ErrDecrypt
	}

	if err := json.Unmarshal(plaintext, &s.identities); err != nil {
		return nil, fmt.Errorf("corrupt identity data: %w", err)
	}
	return s, nil
}

// save encrypts and writes the identity map to disk.
func (s *FileStore) save() error {
	plaintext, err := json.Marshal(s.identities)
	if err != nil {
		return err
	}
	encrypted, err := Encrypt(s.key, plaintext)
	if err != nil {
		return err
	}
	sf := storeFile{Salt: s.salt, Data: encrypted}
	data, err := json.Marshal(sf)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

// List returns summaries of all stored identities.
func (s *FileStore) List() ([]Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summaries := make([]Summary, 0, len(s.identities))
	for _, id := range s.identities {
		summaries = append(summaries, id.Summarize())
	}
	return summaries, nil
}

// Get returns the identity with the given name, or ErrNotFound.
func (s *FileStore) Get(name string) (*Identity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.identities[name]
	if !ok {
		return nil, ErrNotFound
	}
	return &id, nil
}

// Add stores a new identity. Returns ErrDuplicate if the name already exists.
func (s *FileStore) Add(id Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.identities[id.Name]; exists {
		return ErrDuplicate
	}
	s.identities[id.Name] = id
	return s.save()
}

// Update replaces an existing identity. Returns ErrNotFound if the name does not exist.
func (s *FileStore) Update(name string, id Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.identities[name]; !exists {
		return ErrNotFound
	}
	if name != id.Name {
		delete(s.identities, name)
	}
	s.identities[id.Name] = id
	return s.save()
}

// Remove deletes an identity by name. Returns ErrNotFound if it does not exist.
func (s *FileStore) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.identities[name]; !exists {
		return ErrNotFound
	}
	delete(s.identities, name)
	return s.save()
}
