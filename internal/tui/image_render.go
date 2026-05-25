package tui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"  // register GIF decoder for image.Decode
	_ "image/jpeg" // register JPEG decoder for image.Decode
	_ "image/png"  // register PNG decoder for image.Decode
	"strings"
)

// renderCoverArt converts raw image bytes (JPEG, PNG or GIF) into a coloured
// half-block character preview suitable for embedding in a terminal viewport.
//
// The algorithm mirrors ansify's approach:
//   - Scale the image to fit maxW columns and maxH character rows; each
//     character cell represents a 2×1 pixel block (upper half-block ▀ with
//     foreground colour for the top pixel row, background colour for the
//     bottom pixel row).
//   - Colours use ANSI 24-bit true-colour escape sequences.
//   - The rendered image is centred inside the maxW×maxH area.
//
// Returns an empty string when data is nil/empty, the image cannot be decoded,
// or the available space is too small to render anything useful.
func renderCoverArt(data []byte, maxW, maxH int) string {
	if maxW < 4 || maxH < 2 || len(data) == 0 {
		return ""
	}

	// ── 1. Decode image ───────────────────────────────────────────────────
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ""
	}

	bounds := img.Bounds()
	srcW := bounds.Max.X - bounds.Min.X
	srcH := bounds.Max.Y - bounds.Min.Y
	if srcW <= 0 || srcH <= 0 {
		return ""
	}

	// ── 2. Compute target dimensions (contain / letterbox) ───────────────
	// Terminal character cells are roughly 2× taller than wide, so each
	// character row covers 2 pixel rows.  We want the largest cols×rows that:
	//   a) fits within maxW columns and maxH character rows, and
	//   b) preserves the original image aspect ratio.
	//
	// Two candidate fits:
	//   width-first:  cols = maxW  → rows = maxW * srcH / (srcW * 2)
	//   height-first: rows = maxH  → cols = maxH * 2 * srcW / srcH
	// Pick whichever candidate stays within both limits.
	colsW := maxW
	rowsW := maxW * srcH / (srcW * 2)

	colsH := maxH * 2 * srcW / srcH
	rowsH := maxH

	var cols, rows int
	if rowsW <= maxH {
		cols, rows = colsW, rowsW
	} else {
		cols, rows = colsH, rowsH
	}
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	// Target pixel dimensions (2 pixel rows per character row).
	pixW := cols
	pixH := rows * 2

	// ── 3. Nearest-neighbour downscale ────────────────────────────────────
	scaled := image.NewNRGBA(image.Rect(0, 0, pixW, pixH))
	for py := 0; py < pixH; py++ {
		sy := bounds.Min.Y + py*srcH/pixH
		for px := 0; px < pixW; px++ {
			sx := bounds.Min.X + px*srcW/pixW
			r, g, b, a := img.At(sx, sy).RGBA()
			// RGBA() returns pre-multiplied 16-bit values; convert to 8-bit.
			if a == 0 {
				scaled.SetNRGBA(px, py, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
			} else {
				scaled.SetNRGBA(px, py, color.NRGBA{
					R: uint8(r * 0xff / a),
					G: uint8(g * 0xff / a),
					B: uint8(b * 0xff / a),
					A: uint8(a >> 8),
				})
			}
		}
	}

	// ── 4. Render half-block characters (centred) ────────────────────────
	// Pad vertically: prepend (maxH-rows)/2 blank lines so the image sits
	// in the vertical centre of the viewport.
	// Pad horizontally: prepend (maxW-cols)/2 spaces to each row so the
	// image sits in the horizontal centre of the viewport.
	padTop := (maxH - rows) / 2
	padLeft := (maxW - cols) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	padRight := maxW - cols - padLeft // fill remaining columns so every line is exactly maxW wide
	if padRight < 0 {
		padRight = 0
	}
	leftPad := strings.Repeat(" ", padLeft)
	rightPad := strings.Repeat(" ", padRight)

	var sb strings.Builder
	sb.Grow((padTop+rows)*(cols*20+1) + padTop)

	blankLine := strings.Repeat(" ", maxW)
	for i := 0; i < padTop; i++ {
		sb.WriteString(blankLine)
		sb.WriteByte('\n')
	}

	for row := 0; row < rows; row++ {
		topY := row * 2
		botY := topY + 1

		sb.WriteString(leftPad)
		for col := 0; col < cols; col++ {
			top := scaled.NRGBAAt(col, topY)
			bot := scaled.NRGBAAt(col, botY)

			// Compose ANSI 24-bit colour escape: fg=top, bg=bot, char=▀.
			// \033[38;2;R;G;Bm  — set foreground
			// \033[48;2;R;G;Bm  — set background
			fmt.Fprintf(&sb, "\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀",
				top.R, top.G, top.B,
				bot.R, bot.G, bot.B,
			)
		}
		// Reset colour, then pad to full width so the border sees a uniform line length.
		sb.WriteString("\033[0m")
		sb.WriteString(rightPad)
		if row < rows-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}
