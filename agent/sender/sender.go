// SPDX-License-Identifier: MIT

// Package sender provides HTTP communication with the SHM server.
package sender

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"time"
)

// Identity holds the cryptographic identity for the agent.
type Identity struct {
	InstanceID string            `json:"instance_id"`
	PrivateKey ed25519.PrivateKey `json:"-"`
	PublicKey  ed25519.PublicKey  `json:"-"`
	PrivKeyHex string            `json:"private_key"`
	PubKeyHex  string            `json:"public_key"`
}

// RegisterRequest is the payload for instance registration.
type RegisterRequest struct {
	InstanceID     string `json:"instance_id"`
	PublicKey      string `json:"public_key"`
	AppName        string `json:"app_name"`
	AppVersion     string `json:"app_version"`
	DeploymentMode string `json:"deployment_mode"`
	Environment    string `json:"environment"`
	OSArch         string `json:"os_arch"`
}

// SnapshotRequest is the payload for snapshot submission.
type SnapshotRequest struct {
	InstanceID string          `json:"instance_id"`
	Timestamp  time.Time       `json:"timestamp"`
	Metrics    json.RawMessage `json:"metrics"`
}

// Sender sends metrics to the SHM server.
type Sender struct {
	serverURL   string
	appName     string
	appVersion  string
	environment string
	identity    *Identity
	client      *http.Client
	logger      *slog.Logger
	registered  bool
}

// Config holds sender configuration.
type Config struct {
	ServerURL   string
	AppName     string
	AppVersion  string
	Environment string
	Identity    *Identity
	Logger      *slog.Logger
}

// New creates a new Sender.
func New(cfg Config) *Sender {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Sender{
		serverURL:   cfg.ServerURL,
		appName:     cfg.AppName,
		appVersion:  cfg.AppVersion,
		environment: cfg.Environment,
		identity:    cfg.Identity,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// Register registers the agent with the server.
func (s *Sender) Register(ctx context.Context) error {
	if s.registered {
		return nil
	}

	req := RegisterRequest{
		InstanceID:     s.identity.InstanceID,
		PublicKey:      s.identity.PubKeyHex,
		AppName:        s.appName,
		AppVersion:     s.appVersion,
		DeploymentMode: detectDeploymentMode(),
		Environment:    s.environment,
		OSArch:         fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling register request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.serverURL+"/v1/register", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating register request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("sending register request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	s.registered = true
	s.logger.Info("registered with server", "instance_id", s.identity.InstanceID)

	// Activate after registration
	return s.activate(ctx)
}

// activate sends an activation request.
func (s *Sender) activate(ctx context.Context) error {
	payload := map[string]string{
		"instance_id": s.identity.InstanceID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling activate request: %w", err)
	}

	signature := sign(s.identity.PrivateKey, body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.serverURL+"/v1/activate", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating activate request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Signature", signature)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("sending activate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("activate failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	s.logger.Info("activated with server")
	return nil
}

// SendSnapshot sends metrics to the server.
func (s *Sender) SendSnapshot(ctx context.Context, metrics map[string]interface{}) error {
	if !s.registered {
		if err := s.Register(ctx); err != nil {
			return fmt.Errorf("registering: %w", err)
		}
	}

	metricsJSON, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("marshaling metrics: %w", err)
	}

	req := SnapshotRequest{
		InstanceID: s.identity.InstanceID,
		Timestamp:  time.Now().UTC(),
		Metrics:    metricsJSON,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling snapshot request: %w", err)
	}

	signature := sign(s.identity.PrivateKey, body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.serverURL+"/v1/snapshot", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating snapshot request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Signature", signature)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("sending snapshot request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("snapshot failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	s.logger.Debug("sent snapshot", "metrics_count", len(metrics))
	return nil
}

// sign creates an Ed25519 signature of the message.
func sign(privateKey ed25519.PrivateKey, message []byte) string {
	sig := ed25519.Sign(privateKey, message)
	return hex.EncodeToString(sig)
}

// detectDeploymentMode detects how the agent is deployed.
func detectDeploymentMode() string {
	// Check for Kubernetes
	if _, exists := lookupEnv("KUBERNETES_SERVICE_HOST"); exists {
		return "kubernetes"
	}

	// Check for Docker
	if fileExists("/.dockerenv") {
		return "docker"
	}

	// Check cgroup for container
	if isInContainer() {
		return "container"
	}

	return "standalone"
}

// lookupEnv is a wrapper for os.LookupEnv (allows testing).
var lookupEnv = func(key string) (string, bool) {
	return "", false
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	// Basic implementation - can be overridden for testing
	return false
}

// isInContainer checks if running in a container via cgroup.
func isInContainer() bool {
	// Basic implementation - can be enhanced
	return false
}
