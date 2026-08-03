package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

func checkStorageRoot(ctx context.Context, root string) error {
	if ctx == nil {
		return errors.New("storage readiness: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return errors.New("storage readiness: empty root")
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("storage readiness: %w", err)
	}
	if !info.IsDir() {
		return errors.New("storage readiness: root is not a directory")
	}
	return nil
}
