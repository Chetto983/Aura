package mcptools

import (
	"context"
	"log/slog"
)

// BrowserBox is the host side of the browser recipe's calls: admission to the host before a call
// that can open a Chrome, and the download crossing after every call, which keeps
// credential-bearing files out of the shell's transfer surface.
type BrowserBox interface {
	AdmitBrowser(context.Context) error
	CollectBrowserDownloads(context.Context) (string, error)
}

func (b *bridgedTool) downloadFooter(ctx context.Context) string {
	if b.srv.browserBox == nil {
		return ""
	}
	footer, err := b.srv.browserBox.CollectBrowserDownloads(ctx)
	if err != nil {
		slog.Warn("browser download collection failed", "error", err)
		footer += "\nBrowser downloads could not be collected; retry with the next browser call."
	}
	return footer
}
