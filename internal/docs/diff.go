package docs

import (
	"html"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// DiffLine represents a single line in a diff view
type DiffLine struct {
	Type       string // "equal", "insert", "delete"
	Content    string
	OldLineNum int // 0 means N/A
	NewLineNum int // 0 means N/A
}

// DiffResult contains the diff between two texts
type DiffResult struct {
	Lines       []DiffLine
	OldText     string
	NewText     string
	Additions   int
	Deletions   int
}

// ComputeDiff generates a line-by-line diff between two texts
func ComputeDiff(oldText, newText string) *DiffResult {
	dmp := diffmatchpatch.New()

	// Split into lines first for cleaner diff
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	// Use diffmatchpatch for line-based diff
	a, b, lineArray := dmp.DiffLinesToChars(oldText, newText)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)
	diffs = dmp.DiffCleanupSemantic(diffs)

	result := &DiffResult{
		OldText: oldText,
		NewText: newText,
	}

	oldLineNum := 1
	newLineNum := 1

	for _, diff := range diffs {
		lines := strings.Split(strings.TrimSuffix(diff.Text, "\n"), "\n")
		for i, line := range lines {
			// Skip empty trailing line from split
			if i == len(lines)-1 && line == "" && strings.HasSuffix(diff.Text, "\n") {
				continue
			}

			diffLine := DiffLine{
				Content: html.EscapeString(line),
			}

			switch diff.Type {
			case diffmatchpatch.DiffEqual:
				diffLine.Type = "equal"
				diffLine.OldLineNum = oldLineNum
				diffLine.NewLineNum = newLineNum
				oldLineNum++
				newLineNum++
			case diffmatchpatch.DiffDelete:
				diffLine.Type = "delete"
				diffLine.OldLineNum = oldLineNum
				oldLineNum++
				result.Deletions++
			case diffmatchpatch.DiffInsert:
				diffLine.Type = "insert"
				diffLine.NewLineNum = newLineNum
				newLineNum++
				result.Additions++
			}

			result.Lines = append(result.Lines, diffLine)
		}
	}

	// Handle edge case: both texts are empty
	if len(result.Lines) == 0 && len(oldLines) == 1 && oldLines[0] == "" && len(newLines) == 1 && newLines[0] == "" {
		result.Lines = append(result.Lines, DiffLine{
			Type:       "equal",
			Content:    "",
			OldLineNum: 1,
			NewLineNum: 1,
		})
	}

	return result
}
