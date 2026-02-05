package setup

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// RequiredPackages lists the Homebrew packages needed for Ekiden
var RequiredPackages = []string{"tart", "sshpass", "wget"}

// Run performs the initial host machine setup
func Run(log *zap.Logger) error {
	log.Info("Starting host setup")

	// Check/install Homebrew
	if err := ensureHomebrew(log); err != nil {
		return fmt.Errorf("homebrew setup failed: %w", err)
	}

	// Install required packages
	for _, pkg := range RequiredPackages {
		if err := ensurePackage(log, pkg); err != nil {
			return fmt.Errorf("failed to install %s: %w", pkg, err)
		}
	}

	// Validate system
	if err := validateSystem(log); err != nil {
		log.Warn("System validation warnings", zap.Error(err))
	}

	log.Info("Host setup completed successfully")
	fmt.Println("\nSetup complete! You can now run:")
	fmt.Println("  ekiden run --config ekiden.yaml")

	return nil
}

func ensureHomebrew(log *zap.Logger) error {
	// Check if Homebrew is installed
	if _, err := exec.LookPath("brew"); err == nil {
		log.Info("Homebrew is already installed")
		return nil
	}

	log.Info("Installing Homebrew")

	// Install Homebrew
	cmd := exec.Command("/bin/bash", "-c",
		`/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

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

func ensurePackage(log *zap.Logger, pkg string) error {
	// Check if package is installed
	cmd := exec.Command("brew", "list", pkg)
	if err := cmd.Run(); err == nil {
		log.Info("Package already installed", zap.String("package", pkg))
		return nil
	}

	log.Info("Installing package", zap.String("package", pkg))

	// Install package
	cmd = exec.Command("brew", "install", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

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

// CheckDependencies verifies all required tools are available
func CheckDependencies() error {
	var missing []string

	for _, pkg := range RequiredPackages {
		if _, err := exec.LookPath(pkg); err != nil {
			missing = append(missing, pkg)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required tools: %s. Run 'ekiden setup' first", strings.Join(missing, ", "))
	}

	return nil
}
