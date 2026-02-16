package k8s

import (
	"context"
	"fmt"
	"os/exec"
)

func DetectKubectl() error {
	_, err := exec.LookPath("kubectl")
	return err
}

func Apply(ctx context.Context, manifestPath string) error {
	if err := DetectKubectl(); err != nil {
		return fmt.Errorf("kubectl not found in PATH")
	}
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", manifestPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func Delete(ctx context.Context, manifestPath string) error {
	if err := DetectKubectl(); err != nil {
		return fmt.Errorf("kubectl not found in PATH")
	}
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "-f", manifestPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
