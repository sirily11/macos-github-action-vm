package setup

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rxtech-lab/rvmm/assets"
	"go.uber.org/zap"
)

// RequiredPackages lists the Homebrew packages needed for Ekiden
var RequiredPackages = []string{"tart", "sshpass", "wget", "hashicorp/tap/packer"}

// RequiredTools lists the binaries needed at runtime
var RequiredTools = []string{"tart", "sshpass", "wget", "packer"}

// Run performs the initial host machine setup
func Run(log *zap.Logger) error {
	return RunWithIO(log, os.Stdout, os.Stderr, os.Stdin)
}

// RunWithIO performs setup using the provided IO streams.
func RunWithIO(log *zap.Logger, stdout, stderr io.Writer, stdin io.Reader) error {
	log.Info("Starting host setup")

	// Check/install Homebrew
	if err := ensureHomebrew(log, stdout, stderr, stdin); err != nil {
		return fmt.Errorf("homebrew setup failed: %w", err)
	}

	// Install required packages
	if err := ensureTap(log, "hashicorp/tap", stdout, stderr); err != nil {
		return fmt.Errorf("failed to tap hashicorp: %w", err)
	}
	for _, pkg := range RequiredPackages {
		if err := ensurePackage(log, pkg, stdout, stderr); err != nil {
			return fmt.Errorf("failed to install %s: %w", pkg, err)
		}
	}

	// Validate system
	if err := validateSystem(log); err != nil {
		log.Warn("System validation warnings", zap.Error(err))
	}

	if err := createSampleConfig(log); err != nil {
		return fmt.Errorf("create sample config failed: %w", err)
	}

	log.Info("Host setup completed successfully")
	fmt.Fprintln(stdout, "\nSetup complete! You can now use the TUI to run the runner.")

	return nil
}

func createSampleConfig(log *zap.Logger) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	configPath := filepath.Join(workingDir, "rvmm.yaml")
	if _, err := os.Stat(configPath); err == nil {
		log.Info("Sample config already exists", zap.String("path", configPath))
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check existing config: %w", err)
	}

	if err := os.WriteFile(configPath, assets.ConfigExample, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	log.Info("Sample config created", zap.String("path", configPath))
	return nil
}

func ensureHomebrew(log *zap.Logger, stdout, stderr io.Writer, stdin io.Reader) error {
	// Check if Homebrew is installed
	if _, err := exec.LookPath("brew"); err == nil {
		log.Info("Homebrew is already installed")
		return nil
	}

	log.Info("Installing Homebrew")

	// Install Homebrew
	cmd := exec.Command("/bin/bash", "-c",
		`/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("homebrew installation failed: %w", err)
	}

	// Add Homebrew to PATH for Apple Silicon
	if _, err := os.Stat("/opt/homebrew/bin/brew"); err == nil {
		os.Setenv("PATH", "/opt/homebrew/bin:"+os.Getenv("PATH"))
	}

	log.Info("Homebrew installed successfully")
	return nil
}

func ensurePackage(log *zap.Logger, pkg string, stdout, stderr io.Writer) error {
	// Check if package is installed
	cmd := exec.Command("brew", "list", pkg)
	if err := cmd.Run(); err == nil {
		log.Info("Package already installed", zap.String("package", pkg))
		return nil
	}

	log.Info("Installing package", zap.String("package", pkg))

	// Install package
	cmd = exec.Command("brew", "install", pkg)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew install %s failed: %w", pkg, err)
	}

	log.Info("Package installed", zap.String("package", pkg))
	return nil
}

func validateSystem(log *zap.Logger) error {
	var warnings []string

	// Check macOS version
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err == nil {
		version := strings.TrimSpace(string(out))
		log.Info("macOS version", zap.String("version", version))

		// Check for Apple Silicon
		arch, _ := exec.Command("uname", "-m").Output()
		if strings.TrimSpace(string(arch)) != "arm64" {
			warnings = append(warnings, "Tart requires Apple Silicon (arm64)")
		}
	}

	// Check virtualization capability
	cmd := exec.Command("sysctl", "-n", "kern.hv_support")
	out, err = cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "1" {
		warnings = append(warnings, "Hardware virtualization may not be supported")
	}

	// Check disk space
	out, _ = exec.Command("df", "-h", "/").Output()
	log.Info("Disk space", zap.String("output", strings.TrimSpace(string(out))))

	if len(warnings) > 0 {
		return fmt.Errorf("warnings: %s", strings.Join(warnings, "; "))
	}

	return nil
}

func ensureTap(log *zap.Logger, tap string, stdout, stderr io.Writer) error {
	cmd := exec.Command("brew", "tap")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == tap {
				log.Info("Homebrew tap already present", zap.String("tap", tap))
				return nil
			}
		}
	}

	log.Info("Adding Homebrew tap", zap.String("tap", tap))
	cmd = exec.Command("brew", "tap", tap)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew tap %s failed: %w", tap, err)
	}

	return nil
}

// CheckDependencies verifies all required tools are available
func CheckDependencies() error {
	var missing []string

	for _, pkg := range RequiredTools {
		if _, err := exec.LookPath(pkg); err != nil {
			missing = append(missing, pkg)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required tools: %s. Run setup from the TUI first", strings.Join(missing, ", "))
	}

	return nil
}
