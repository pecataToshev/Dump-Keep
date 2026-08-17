package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"filippo.io/age"
	"github.com/pecataToshev/dump-keep/internal/storage"
)

// dumpEncryptUpload runs a dump command and streams
// stdout -> age encryption -> storage, without touching disk.
// On any failure the partially uploaded object is deleted.
func dumpEncryptUpload(
	ctx context.Context,
	store storage.Provider,
	key string,
	recipient age.Recipient,
	command string, args ...string,
) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", command, err)
	}

	pr, pw := io.Pipe()
	go func() {
		encrypted, err := age.Encrypt(pw, recipient)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(encrypted, stdout); err != nil {
			pw.CloseWithError(err)
			return
		}
		// Flush the age stream before closing the pipe.
		if err := encrypted.Close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()

	uploadErr := store.Put(ctx, key, pr)
	dumpErr := cmd.Wait()

	if uploadErr != nil || dumpErr != nil {
		// Best effort: don't leave truncated backups behind.
		if delErr := store.Delete(ctx, key); delErr != nil {
			fmt.Fprintf(os.Stderr, "failed to delete partial upload %s: %v\n", key, delErr)
		}
		if dumpErr != nil {
			return fmt.Errorf("%s failed: %w", command, dumpErr)
		}
		return fmt.Errorf("upload %s: %w", key, uploadErr)
	}

	return nil
}
