package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rxtech-lab/rvmm/internal/config"
	"go.uber.org/zap"
)

var ipRegex = regexp.MustCompile(`^(\d+\.){3}\d+$`)

// VMManager handles Tart VM operations
type VMManager struct {
	cfg *config.Config
	log *zap.Logger
	// Resolved image ref to use for clone/run (local or registry)
	imageRef string
}

// NewVMManager creates a new VM manager
func NewVMManager(cfg *config.Config, log *zap.Logger) *VMManager {
	return &VMManager{
		cfg: cfg,
		log: log,
	}
}

// GetRegistryPath returns the full image path for tart commands
func (v *VMManager) GetRegistryPath() string {
	if v.cfg.Registry.URL == "" {
		return v.cfg.Registry.ImageName
	}

	// Avoid double-prefixing when image name already includes the registry host
	registryPrefix := v.cfg.Registry.URL + "/"
	if strings.HasPrefix(v.cfg.Registry.ImageName, registryPrefix) {
		return v.cfg.Registry.ImageName
	}

	return fmt.Sprintf("%s/%s", v.cfg.Registry.URL, v.cfg.Registry.ImageName)
}

// GetCachePath returns the local cache path for the image
func (v *VMManager) GetCachePath() string {
	registryPath := v.GetRegistryPath()
	// Replace : with / for tag in path
	cachePath := strings.ReplaceAll(registryPath, ":", "/")
	return filepath.Join(os.Getenv("HOME"), ".tart", "cache", "OCIs", cachePath)
}

// Login authenticates with the registry if credentials are provided
func (v *VMManager) Login(ctx context.Context) error {
	if v.cfg.Registry.URL == "" || v.cfg.Registry.Username == "" {
		return nil
	}

	v.log.Info("Logging in to registry", zap.String("url", v.cfg.Registry.URL))

	cmd := exec.CommandContext(ctx, "tart", "login", v.cfg.Registry.URL)
	cmd.Env = append(os.Environ(),
		"TART_REGISTRY_USERNAME="+v.cfg.Registry.Username,
		"TART_REGISTRY_PASSWORD="+v.cfg.Registry.Password,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("registry login failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// ImageExists checks if the image is already cached locally
func (v *VMManager) ImageExists(ctx context.Context) (bool, error) {
	localRef := v.cfg.Registry.ImageName
	registryPath := v.GetRegistryPath()
	localName := localRef
	if idx := strings.Index(localRef, ":"); idx > 0 {
		localName = localRef[:idx]
	}

	cmd := exec.CommandContext(ctx, "tart", "list")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("tart list failed: %w", err)
	}

	listOutput := string(output)
	if strings.Contains(listOutput, localRef) {
		v.imageRef = localRef
		return true, nil
	}
	if localName != localRef && strings.Contains(listOutput, localName) {
		v.imageRef = localName
		return true, nil
	}
	if registryPath != localRef && strings.Contains(listOutput, registryPath) {
		v.imageRef = registryPath
		return true, nil
	}

	// Default to registry path for pulls/clones when not found locally
	v.imageRef = registryPath
	return false, nil
}

// PullImage pulls the image from the registry
func (v *VMManager) PullImage(ctx context.Context) error {
	v.log.Info("Pulling VM image from registry")

	// Remove old cached images
	v.log.Info("Removing old cached images")
	tartDir := filepath.Join(os.Getenv("HOME"), ".tart")
	if err := os.RemoveAll(tartDir); err != nil {
		v.log.Warn("Failed to remove old tart directory", zap.Error(err))
	}

	registryPath := v.GetRegistryPath()

	cmd := exec.CommandContext(ctx, "tart", "pull", registryPath, "--concurrency", "1")

	if v.cfg.Registry.Username != "" {
		cmd.Env = append(os.Environ(),
			"TART_REGISTRY_USERNAME="+v.cfg.Registry.Username,
			"TART_REGISTRY_PASSWORD="+v.cfg.Registry.Password,
		)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tart pull failed: %w", err)
	}

	// Resize disk if configured
	if v.cfg.Options.TruncateSize != "" {
		if err := v.resizeCachedImage(ctx); err != nil {
			return fmt.Errorf("disk resize failed: %w", err)
		}
	}

	return nil
}

func (v *VMManager) resizeCachedImage(ctx context.Context) error {
	v.log.Info("Resizing cached image disk", zap.String("size", v.cfg.Options.TruncateSize))

	diskPath := filepath.Join(v.GetCachePath(), "disk.img")

	// Truncate disk file
	cmd := exec.CommandContext(ctx, "truncate", "-s", v.cfg.Options.TruncateSize, diskPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("truncate failed: %w", err)
	}

	// Boot temp VM to resize partition
	tempInstance := "truncate_instance"
	registryPath := v.GetRegistryPath()

	// Clone for resize
	cmd = exec.CommandContext(ctx, "tart", "clone", registryPath, tempInstance)
	cmd.Env = append(os.Environ(), "TART_NO_AUTO_PRUNE=")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clone for resize failed: %w", err)
	}

	defer func() {
		exec.CommandContext(ctx, "tart", "stop", tempInstance).Run()
		exec.CommandContext(ctx, "tart", "delete", tempInstance).Run()
	}()

	// Boot VM
	bootCmd := exec.CommandContext(ctx, "tart", "run", "--no-graphics", tempInstance)
	if err := bootCmd.Start(); err != nil {
		return fmt.Errorf("failed to start temp VM: %w", err)
	}

	// Wait for IP
	ip, err := v.waitForIP(ctx, tempInstance)
	if err != nil {
		return fmt.Errorf("failed to get temp VM IP: %w", err)
	}

	// Wait for SSH
	ssh := NewSSHClient(v.cfg, v.log)
	if err := ssh.WaitForSSH(ctx, ip); err != nil {
		return fmt.Errorf("SSH not available on temp VM: %w", err)
	}

	// Repair and resize disk
	ssh.Execute(ctx, ip, "echo y | diskutil repairDisk disk0", false)
	ssh.Execute(ctx, ip, "diskutil apfs resizeContainer disk0s2 0", false)

	// Stop VM
	exec.CommandContext(ctx, "tart", "stop", tempInstance).Run()

	// Copy resized disk back
	tempDiskPath := filepath.Join(os.Getenv("HOME"), ".tart", "vms", tempInstance, "disk.img")
	os.Remove(diskPath)
	cmd = exec.CommandContext(ctx, "cp", "-c", tempDiskPath, diskPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy resized disk: %w", err)
	}

	v.log.Info("Disk resized successfully")
	return nil
}

// Clone creates a new VM instance from the cached image
func (v *VMManager) Clone(ctx context.Context, instanceName string) error {
	v.log.Info("Cloning VM", zap.String("instance", instanceName))

	imageRef := v.imageRef
	if imageRef == "" {
		imageRef = v.GetRegistryPath()
	}

	cmd := exec.CommandContext(ctx, "tart", "clone", imageRef, instanceName)
	cmd.Env = append(os.Environ(), "TART_NO_AUTO_PRUNE=")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tart clone failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Start boots a VM instance
func (v *VMManager) Start(ctx context.Context, instanceName string) (*exec.Cmd, error) {
	v.log.Info("Starting VM", zap.String("instance", instanceName))

	cmd := exec.CommandContext(ctx, "tart", "run", "--no-graphics", instanceName)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("tart run failed: %w", err)
	}

	return cmd, nil
}

// WaitForIP polls until the VM has an IP address
func (v *VMManager) WaitForIP(ctx context.Context, instanceName string) (string, error) {
	return v.waitForIP(ctx, instanceName)
}

func (v *VMManager) waitForIP(ctx context.Context, instanceName string) (string, error) {
	v.log.Info("Waiting for VM IP address", zap.String("instance", instanceName))

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for VM IP")
		case <-ticker.C:
			cmd := exec.CommandContext(ctx, "tart", "ip", instanceName)
			output, err := cmd.Output()
			if err != nil {
				continue
			}

			ip := strings.TrimSpace(string(output))
			if ipRegex.MatchString(ip) {
				v.log.Info("VM IP obtained", zap.String("ip", ip))

				// Remove old SSH host key
				exec.Command("ssh-keygen", "-R", ip).Run()

				return ip, nil
			}
		}
	}
}

// Stop stops a running VM instance
func (v *VMManager) Stop(ctx context.Context, instanceName string) error {
	v.log.Info("Stopping VM", zap.String("instance", instanceName))

	cmd := exec.CommandContext(ctx, "tart", "stop", instanceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tart stop failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Delete removes a VM instance
func (v *VMManager) Delete(ctx context.Context, instanceName string) error {
	v.log.Info("Deleting VM", zap.String("instance", instanceName))

	cmd := exec.CommandContext(ctx, "tart", "delete", instanceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tart delete failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Cleanup stops and deletes a VM instance, ignoring errors
func (v *VMManager) Cleanup(ctx context.Context, instanceName string) {
	v.log.Info("Cleaning up VM", zap.String("instance", instanceName))

	// Use a fresh context for cleanup in case the original was cancelled
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	v.Stop(cleanupCtx, instanceName)
	v.Delete(cleanupCtx, instanceName)
}
