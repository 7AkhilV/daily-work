package clipboard

import (
	"fmt"

	"github.com/atotto/clipboard"
)

func Copy(text string) error {
	if err := clipboard.WriteAll(text); err != nil {
		return fmt.Errorf("copy to clipboard: %w", err)
	}
	return nil
}
