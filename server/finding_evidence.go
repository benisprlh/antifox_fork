package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif" // Register GIF decoder.
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"antifox/report"
)

// Finding evidence upload/display/management. Files live under report_assets/evidence/<node_id>/.
// Findings use the cookie-authenticated evidence endpoint; reports embed files from disk.
//
// Upload security requirements:
// Validate content and decode actual images instead of trusting extension/client MIME;
// re-encode as clean PNG/JPEG to strip appended payloads, EXIF, and polyglot content.
// Check DecodeConfig dimensions before decoding to prevent decompression bombs.
// Serve fixed image Content-Type with nosniff and inline disposition to prevent stored XSS.

const (
	evidenceMaxBytes  = 16 << 20   // 16 MB per image.
	evidenceMaxPixels = 50_000_000 // 50 million pixels; decompression-bomb limit.
	evidenceMaxDim    = 20000
)

// sanitizeImage validates PNG/JPEG/GIF content and re-encodes as PNG or JPEG.
func sanitizeImage(raw []byte) (out []byte, ext string, err error) {
	ct := http.DetectContentType(raw)
	switch ct {
	case "image/png", "image/jpeg", "image/gif":
	default:
		return nil, "", fmt.Errorf("only PNG/JPEG/GIF images are supported (detected %s)", ct)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("invalid image")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > evidenceMaxDim || cfg.Height > evidenceMaxDim ||
		cfg.Width*cfg.Height > evidenceMaxPixels {
		return nil, "", fmt.Errorf("image dimensions exceed the limit")
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("image decoding failed")
	}
	var buf bytes.Buffer
	if ct == "image/jpeg" {
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, "", fmt.Errorf("image processing failed")
		}
		return buf.Bytes(), ".jpg", nil
	}
	if err := png.Encode(&buf, img); err != nil {
		return nil, "", fmt.Errorf("image processing failed")
	}
	return buf.Bytes(), ".png", nil
}

// evidenceDir returns the absolute directory for a finding's evidence files.
func (s *Server) evidenceDir(nodeID int64) string {
	return filepath.Join(s.dataDir, "report_assets", "evidence", strconv.FormatInt(nodeID, 10))
}

// evidenceURL constructs the frontend image URL.
func evidenceURL(nodeID int64, filename string) string {
	return fmt.Sprintf("/api/findings/%d/evidence/%s", nodeID, filename)
}

// POST /api/findings/{id}/evidence: multipart file with optional caption/step_index.
func (s *Server) uploadFindingEvidence(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	nodeID, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "Invalid finding ID")
		return
	}
	if err := r.ParseMultipartForm(16 << 20); err != nil { // 16 MB
		writeErr(w, 400, "Failed to parse upload")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, 400, "Missing file")
		return
	}
	defer file.Close()
	// Read up to 16 MB, validate content, and re-encode; do not trust extension/client MIME.
	raw, err := io.ReadAll(io.LimitReader(file, evidenceMaxBytes+1))
	if err != nil {
		writeErr(w, 400, "Failed to read file")
		return
	}
	if len(raw) > evidenceMaxBytes {
		writeErr(w, 400, "Image exceeds the 16 MB limit")
		return
	}
	clean, ext, err := sanitizeImage(raw)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	dir := s.evidenceDir(nodeID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, 500, "Failed to create directory")
		return
	}
	name := fmt.Sprintf("ev-%d%s", time.Now().UnixNano(), ext)
	if err := os.WriteFile(filepath.Join(dir, name), clean, 0o644); err != nil {
		writeErr(w, 500, "Write failed")
		return
	}
	caption := strings.TrimSpace(r.FormValue("caption"))
	stepIndex, _ := strconv.Atoi(r.FormValue("step_index"))
	if caption == "" {
		// Derive editable default captions from agent-recorded reproduction steps without another LLM call.
		caption = s.deriveEvidenceCaption(nodeID, stepIndex)
	}
	ev, err := pg.CreateFindingEvidence(nodeID, name, caption, stepIndex)
	if err != nil {
		_ = os.Remove(filepath.Join(dir, name)) // Roll back the file write.
		writeErr(w, 500, "Save failed: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"evidence": ev, "url": evidenceURL(nodeID, name)})
}

// deriveEvidenceCaption builds a default figure caption for an evidence shot from the
// The caption derives from the recorded step and what it proves, without another LLM call.
// It remains editable; step 0 (overview) or missing steps return an empty string.
func (s *Server) deriveEvidenceCaption(nodeID int64, stepIndex int) string {
	if stepIndex <= 0 || s.m == nil || s.m.pg == nil {
		return ""
	}
	f, err := s.m.pg.GetFindingByNodeID(nodeID)
	if err != nil || f == nil || len(f.Reproduction) == 0 {
		return ""
	}
	var rp report.Reproduction
	if json.Unmarshal(f.Reproduction, &rp) != nil || stepIndex > len(rp.Steps) {
		return ""
	}
	st := rp.Steps[stepIndex-1]
	text := strings.TrimSpace(st.Proves)
	if text == "" {
		text = strings.TrimSpace(st.Expected)
	}
	if text == "" {
		return ""
	}
	text = strings.SplitN(text, "\n", 2)[0] // Single-line thumbnail caption.
	return clipUTF8(fmt.Sprintf("Step %d: %s", stepIndex, text), 120)
}

// GET /api/findings/{id}/evidence lists evidence with image URLs.
func (s *Server) listFindingEvidence(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	nodeID, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "Invalid finding ID")
		return
	}
	list, err := pg.ListFindingEvidence(nodeID)
	if err != nil {
		writeErr(w, 500, "Retrieval failed")
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		out = append(out, map[string]any{"evidence": e, "url": evidenceURL(e.NodeID, e.Filename)})
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

// PATCH /api/findings/{id}/evidence/{eid} updates caption/step/order.
func (s *Server) updateFindingEvidence(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	eid, ok := pathInt(r, "eid")
	if !ok {
		writeErr(w, 400, "Invalid evidence ID")
		return
	}
	var req struct {
		Caption   string `json:"caption"`
		StepIndex int    `json:"step_index"`
		SortOrder int    `json:"sort_order"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, 400, "Invalid request format")
		return
	}
	if err := pg.UpdateFindingEvidence(eid, req.Caption, req.StepIndex, req.SortOrder); err != nil {
		writeErr(w, 500, "Save failed: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// DELETE /api/findings/{id}/evidence/{eid} removes metadata and file.
func (s *Server) deleteFindingEvidence(w http.ResponseWriter, r *http.Request) {
	pg := s.pg(w)
	if pg == nil {
		return
	}
	eid, ok := pathInt(r, "eid")
	if !ok {
		writeErr(w, 400, "Invalid evidence ID")
		return
	}
	ev, err := pg.GetFindingEvidence(eid)
	if err != nil {
		writeErr(w, 404, "Evidence not found")
		return
	}
	if err := pg.DeleteFindingEvidence(eid); err != nil {
		writeErr(w, 500, "Deletion failed: "+err.Error())
		return
	}
	_ = os.Remove(filepath.Join(s.evidenceDir(ev.NodeID), filepath.Base(ev.Filename)))
	writeJSON(w, 200, map[string]any{"deleted": eid})
}

// GET evidence by base filename prevents traversal and uses cookie authentication.
func (s *Server) serveFindingEvidence(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := pathInt(r, "id")
	if !ok {
		writeErr(w, 400, "Invalid finding ID")
		return
	}
	file := filepath.Base(r.PathValue("file"))
	// Fixed image Content-Type, nosniff, and inline disposition prevent HTML/script interpretation.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch strings.ToLower(filepath.Ext(file)) {
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	default:
		writeErr(w, 404, "not found")
		return
	}
	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, filepath.Join(s.evidenceDir(nodeID), file))
}
