// SPDX-License-Identifier: MIT

// Package identity provides cryptographic identity management for the agent.
package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kolapsis/shm-agent/agent/sender"
)

// storedIdentity is the JSON structure for identity persistence.
type storedIdentity struct {
	InstanceID string `json:"instance_id"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

// LoadOrGenerate loads an existing identity or generates a new one.
func LoadOrGenerate(path string) (*sender.Identity, error) {
	// Try to load existing identity
	identity, err := Load(path)
	if err == nil {
		return identity, nil
	}

	// If file doesn't exist, generate new identity
	if os.IsNotExist(err) {
		return Generate(path)
	}

	return nil, fmt.Errorf("loading identity: %w", err)
}

// Load loads an identity from a file.
func Load(path string) (*sender.Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var stored storedIdentity
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("parsing identity file: %w", err)
	}

	privateKey, err := hex.DecodeString(stored.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("decoding private key: %w", err)
	}

	publicKey, err := hex.DecodeString(stored.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decoding public key: %w", err)
	}

	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: got %d, want %d", len(privateKey), ed25519.PrivateKeySize)
	}

	if len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: got %d, want %d", len(publicKey), ed25519.PublicKeySize)
	}

	return &sender.Identity{
		InstanceID: stored.InstanceID,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		PrivKeyHex: stored.PrivateKey,
		PubKeyHex:  stored.PublicKey,
	}, nil
}

// Generate creates a new identity and saves it to a file.
func Generate(path string) (*sender.Identity, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("creating identity directory: %w", err)
	}

	// Generate Ed25519 keypair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating keypair: %w", err)
	}

	// Generate instance ID (UUID-like)
	instanceID, err := generateUUID()
	if err != nil {
		return nil, fmt.Errorf("generating instance ID: %w", err)
	}

	identity := &sender.Identity{
		InstanceID: instanceID,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		PrivKeyHex: hex.EncodeToString(privateKey),
		PubKeyHex:  hex.EncodeToString(publicKey),
	}

	// Save to file
	if err := Save(path, identity); err != nil {
		return nil, err
	}

	return identity, nil
}

// Save saves an identity to a file.
func Save(path string, identity *sender.Identity) error {
	stored := storedIdentity{
		InstanceID: identity.InstanceID,
		PrivateKey: identity.PrivKeyHex,
		PublicKey:  identity.PubKeyHex,
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling identity: %w", err)
	}

	// Write with restricted permissions (owner read/write only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing identity file: %w", err)
	}

	return nil
}

// generateUUID generates a UUID v4.
func generateUUID() (string, error) {
	uuid := make([]byte, 16)
	if _, err := rand.Read(uuid); err != nil {
		return "", err
	}

	// Set version (4) and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant 10

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	), nil
}
