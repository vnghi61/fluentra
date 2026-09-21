package rendition

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	OfficeTimeout   = 120 * time.Second
	ToolLibreOffice = "libreoffice:soffice"
)

// RenderOffice converts Office documents (DOCX, PPTX, etc.) to PDF via headless LibreOffice,
// then delegates to RenderPDF for page rendering.
func RenderOffice(ctx context.Context, sofficeBin, pdftoppmBin string, req RenderRequest) (*RenderResult, error) {
	if sofficeBin == "" {
		p, err := exec.LookPath("soffice")
		if err != nil {
			return &RenderResult{
				Skipped:     true,
				SkipReason:  "soffice (LibreOffice) is not installed or available on PATH",
				ToolVersion: "soffice:missing",
			}, nil
		}
		sofficeBin = p
	}

	execCtx, cancel := context.WithTimeout(ctx, OfficeTimeout)
	defer cancel()

	// 1. Create fresh isolated user profile directory (Trap 6: prevents concurrent collisions)
	profileDir := filepath.Join(req.TempDir, fmt.Sprintf("lo-%s", uuid.New().String()))
	if err := os.MkdirAll(profileDir, 0700); err != nil {
		return nil, fmt.Errorf("create libreoffice profile dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(profileDir) }()

	// Convert profile path to file:/// URL format
	profileURI := "file:///" + filepath.ToSlash(filepath.Clean(profileDir))

	// 2. Convert to PDF: soffice --headless --convert-to pdf --outdir <tmpDir> -env:UserInstallation=... <sourcePath>
	cmd := exec.CommandContext(execCtx, sofficeBin,
		"--headless",
		"--convert-to", "pdf",
		"--outdir", req.TempDir,
		fmt.Sprintf("-env:UserInstallation=%s", profileURI),
		req.SourcePath,
	)

	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("soffice conversion failed: %w (output: %s)", err, strings.TrimSpace(string(outBytes)))
	}

	// 3. Locate the converted PDF
	srcBase := filepath.Base(req.SourcePath)
	srcExt := filepath.Ext(srcBase)
	baseWithoutExt := strings.TrimSuffix(srcBase, srcExt)
	convertedPDF := filepath.Join(req.TempDir, baseWithoutExt+".pdf")

	if _, err := os.Stat(convertedPDF); err != nil {
		// Try case-insensitive or glob match
		matches, gerr := filepath.Glob(filepath.Join(req.TempDir, "*.pdf"))
		if gerr != nil || len(matches) == 0 {
			return nil, fmt.Errorf("soffice did not produce expected pdf at %s", convertedPDF)
		}
		convertedPDF = matches[0]
	}

	// 4. Delegate to RenderPDF for thumbnail / preview extraction and text extraction
	pdfReq := RenderRequest{
		ResourceID:   req.ResourceID,
		Kind:         req.Kind,
		SourcePath:   convertedPDF,
		SourceMIME:   "application/pdf",
		TempDir:      req.TempDir,
		PDFToTextBin: req.PDFToTextBin,
	}

	return RenderPDF(ctx, pdftoppmBin, pdfReq)
}
