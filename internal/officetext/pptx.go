package officetext

import (
	"archive/zip"
	"bytes"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// ExtractPPTX converts a slide deck to markdown: one "## Slide N" section per
// slide, with the slide's title (if any) promoted to a heading and other
// text placeholders/tables rendered below it. Slide order follows the
// presentation's actual slide order (ppt/presentation.xml's sldIdLst,
// resolved through the rels part), not filename sort - a deck whose slides
// were reordered after slideN.xml files were first created would otherwise
// come out in the wrong order.
//
// slideCount is the deck's total slide count (including slides that render
// to nothing, e.g. a slide that's entirely an image with no text shapes) -
// the caller (officetext.Extract) uses it to flag decks whose extracted
// text is sparse relative to how many slides actually exist, not just
// whether it's literally empty.
func ExtractPPTX(data []byte) (markdown string, slideCount int, err error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", 0, fmt.Errorf("not a valid PPTX (zip open failed): %w", err)
	}

	slidePaths, err := orderedSlidePaths(zr)
	if err != nil {
		return "", 0, err
	}
	if len(slidePaths) == 0 {
		return "", 0, fmt.Errorf("not a valid PPTX: no slides found")
	}

	var sb strings.Builder
	for i, slidePath := range slidePaths {
		f := findZipFile(zr, slidePath)
		if f == nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		tree, err := parseXMLTree(rc)
		rc.Close()
		if err != nil {
			continue
		}

		slideMD := renderSlide(tree)
		if strings.TrimSpace(slideMD) == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("## Slide %d\n\n", i+1))
		sb.WriteString(slideMD)
	}

	return strings.TrimSpace(sb.String()), len(slidePaths), nil
}

// orderedSlidePaths resolves ppt/presentation.xml's <p:sldId> list (in
// document order) to zip entry paths via ppt/_rels/presentation.xml.rels.
// Falls back to a numeric sort of ppt/slides/slideN.xml if either part is
// missing or malformed, so a still-valid-but-unusual PPTX degrades to
// filename order instead of failing outright.
func orderedSlidePaths(zr *zip.Reader) ([]string, error) {
	presFile := findZipFile(zr, "ppt/presentation.xml")
	relsFile := findZipFile(zr, "ppt/_rels/presentation.xml.rels")
	if presFile == nil || relsFile == nil {
		return fallbackSlidePaths(zr), nil
	}

	presRC, err := presFile.Open()
	if err != nil {
		return fallbackSlidePaths(zr), nil
	}
	presTree, err := parseXMLTree(presRC)
	presRC.Close()
	if err != nil {
		return fallbackSlidePaths(zr), nil
	}

	relsRC, err := relsFile.Open()
	if err != nil {
		return fallbackSlidePaths(zr), nil
	}
	relsTree, err := parseXMLTree(relsRC)
	relsRC.Close()
	if err != nil {
		return fallbackSlidePaths(zr), nil
	}

	targets := make(map[string]string) // rId -> "ppt/slides/slideN.xml"
	for _, rel := range findAll(relsTree, "Relationship") {
		id := rel.attr("Id")
		target := rel.attr("Target")
		if id == "" || target == "" {
			continue
		}
		targets[id] = path.Join("ppt", target)
	}

	sldIDLst := findFirst(presTree, "sldIdLst")
	if sldIDLst == nil {
		return fallbackSlidePaths(zr), nil
	}

	var paths []string
	for _, sldID := range findAll(sldIDLst, "sldId") {
		// <p:sldId id="256" r:id="rId2"/> - "id" and "r:id" collide on local
		// name, so this must go through nsAttr, not attr, to reliably get
		// the relationship id rather than the arbitrary sldId number.
		rID := sldID.nsAttr("id")
		if target, ok := targets[rID]; ok {
			paths = append(paths, target)
		}
	}
	if len(paths) == 0 {
		return fallbackSlidePaths(zr), nil
	}
	return paths, nil
}

func fallbackSlidePaths(zr *zip.Reader) []string {
	type indexed struct {
		n    int
		path string
	}
	var slides []indexed
	for _, f := range zr.File {
		dir := path.Dir(f.Name)
		base := path.Base(f.Name)
		if dir != "ppt/slides" || !strings.HasPrefix(base, "slide") || !strings.HasSuffix(base, ".xml") {
			continue
		}
		numPart := strings.TrimSuffix(strings.TrimPrefix(base, "slide"), ".xml")
		n, err := strconv.Atoi(numPart)
		if err != nil {
			continue
		}
		slides = append(slides, indexed{n: n, path: f.Name})
	}
	for i := 0; i < len(slides); i++ {
		for j := i + 1; j < len(slides); j++ {
			if slides[j].n < slides[i].n {
				slides[i], slides[j] = slides[j], slides[i]
			}
		}
	}
	paths := make([]string, len(slides))
	for i, s := range slides {
		paths[i] = s.path
	}
	return paths
}

// renderSlide walks a slide's shape tree (p:cSld/p:spTree), promoting the
// title placeholder (if any) to a heading and rendering other text shapes as
// bullet lines, tables as markdown tables.
func renderSlide(slideTree *node) string {
	spTree := findFirst(slideTree, "spTree")
	if spTree == nil {
		return ""
	}

	var title string
	var body strings.Builder

	for _, shape := range spTree.Children {
		switch shape.Local {
		case "sp":
			text, isTitle := shapeText(shape)
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			if isTitle && title == "" {
				title = text
				continue
			}
			for _, line := range strings.Split(text, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				body.WriteString("- ")
				body.WriteString(line)
				body.WriteByte('\n')
			}
		case "graphicFrame":
			if tbl := findFirst(shape, "tbl"); tbl != nil {
				writeMarkdownTable(tbl, &body)
			}
		}
	}

	var out strings.Builder
	if title != "" {
		out.WriteString("### ")
		out.WriteString(title)
		out.WriteString("\n\n")
	}
	out.WriteString(body.String())
	out.WriteByte('\n')
	return out.String()
}

// shapeText extracts a shape's text body (p:txBody -> a:p -> a:r/a:t) and
// reports whether the shape is a title/centered-title placeholder
// (p:nvSpPr/p:nvPr/p:ph[@type=title|ctrTitle]).
func shapeText(sp *node) (text string, isTitle bool) {
	if nvSpPr := sp.child("nvSpPr"); nvSpPr != nil {
		if nvPr := nvSpPr.child("nvPr"); nvPr != nil {
			if ph := nvPr.child("ph"); ph != nil {
				t := ph.attr("type")
				isTitle = t == "title" || t == "ctrTitle"
			}
		}
	}

	txBody := sp.child("txBody")
	if txBody == nil {
		return "", isTitle
	}

	var lines []string
	for _, p := range txBody.Children {
		if p.Local != "p" {
			continue
		}
		line := strings.TrimSpace(p.text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), isTitle
}
