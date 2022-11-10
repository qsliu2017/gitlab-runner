package common

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/denisbrodbeck/machineid"
	"github.com/sirupsen/logrus"
)

type LocalConfig struct {
	SystemID string    `toml:"system_id,omitempty" json:"system_id"`
	ModTime  time.Time `toml:"-"`
	Loaded   bool      `toml:"-"`
}

func NewLocalConfig() *LocalConfig {
	return &LocalConfig{}
}

func (c *LocalConfig) StatConfig(configFile string) error {
	_, err := os.Stat(configFile)
	if err != nil {
		return err
	}
	return nil
}

func (c *LocalConfig) LoadConfig(configFile string) error {
	info, err := os.Stat(configFile)

	// permission denied is soft error
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	if _, err = toml.DecodeFile(configFile, c); err != nil {
		return err
	}

	c.ModTime = info.ModTime()
	c.Loaded = true
	return nil
}

func (c *LocalConfig) SaveConfig(configFile string) error {
	var newConfig bytes.Buffer
	newBuffer := bufio.NewWriter(&newConfig)

	if err := toml.NewEncoder(newBuffer).Encode(c); err != nil {
		logrus.Fatalf("Error encoding TOML: %s", err)
		return err
	}

	if err := newBuffer.Flush(); err != nil {
		return err
	}

	// create directory to store configuration
	err := os.MkdirAll(filepath.Dir(configFile), 0700)
	if err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// write config file
	err = os.WriteFile(configFile, newConfig.Bytes(), 0o600)
	if err != nil {
		return fmt.Errorf("saving the local config file: %w", err)
	}

	c.Loaded = true
	return nil
}

func (c *LocalConfig) EnsureSystemID() error {
	if c.SystemID != "" {
		return nil
	}

	if systemID, err := generateUniqueSystemID(); err == nil {
		logrus.WithField("system_id", systemID).Info("Created missing unique system ID")

		c.SystemID = systemID
	} else {
		return fmt.Errorf("generating unique system ID: %w", err)
	}

	return nil
}

func generateUniqueSystemID() (string, error) {
	const idLength = 12

	systemID, err := machineid.ID()
	if err == nil && systemID != "" {
		mac := hmac.New(sha256.New, []byte(systemID))
		mac.Write([]byte("gitlab-runner"))
		systemID = hex.EncodeToString(mac.Sum(nil))
		return "s_" + systemID[0:idLength], nil
	}

	// fallback to a random ID
	return generateRandomSystemID(idLength)
}

func generateRandomSystemID(idLength int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, idLength)
	max := big.NewInt(int64(len(charset)))

	for i := range b {
		r, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}

		b[i] = charset[r.Int64()]
	}
	return "r_" + string(b), nil
}
