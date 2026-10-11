package mcptools

import (
	"context"
	"log/slog"
)

// BrowserDownloads keeps credential-bearing files out of the shell's transfer surface.
type BrowserDownloads interface {
	CollectBrowserDownloads(context.Context) (string, error)
}

func (b *bridgedTool) downloadFooter(ctx context.Context) string {
	if b.srv.downloads == nil {
		return ""
	}
	footer, err := b.srv.downloads.CollectBrowserDownloads(ctx)
	if err != nil {
		slog.Warn("browser download collection failed", "error", err)
		footer += "\nBrowser downloads could not be collected; retry with the next browser call."
	}
	return footer
}
