package diff

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// Opcode represents a diff operation
type Opcode struct {
	Tag    string // "equal", "insert", "replace", "delete"
	I1, I2 int    // old range [I1, I2)
	J1, J2 int    // new range [J1, J2)
}

// LineDiff computes line-level diff between old and new content
func LineDiff(oldContent, newContent []byte) []Opcode {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	// Use line-mode trick: encode each line as a single rune
	dmp := diffmatchpatch.New()
	a, b, lineArray := dmp.DiffLinesToRunes(
		joinLines(oldLines),
		joinLines(newLines),
	)

	diffs := dmp.DiffMainRunes(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	// Convert diffs to opcode format
	return diffsToOpcodes(diffs, oldLines, newLines)
}

func splitLines(content []byte) []string {
	if len(content) == 0 {
		return []string{}
	}

	// Split on \n, preserve empty lines
	lines := strings.Split(string(content), "\n")

	// Remove trailing empty line if file ends with \n
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func diffsToOpcodes(diffs []diffmatchpatch.Diff, oldLines, newLines []string) []Opcode {
	var opcodes []Opcode
	i1, i2 := 0, 0 // indices in old
	j1, j2 := 0, 0 // indices in new

	for _, diff := range diffs {
		// Count lines in this diff segment
		lineCount := strings.Count(diff.Text, "\n")

		switch diff.Type {
		case diffmatchpatch.DiffEqual:
			i1, i2 = i2, i2+lineCount
			j1, j2 = j2, j2+lineCount
			opcodes = append(opcodes, Opcode{
				Tag: "equal",
				I1:  i1,
				I2:  i2,
				J1:  j1,
				J2:  j2,
			})

		case diffmatchpatch.DiffDelete:
			i1, i2 = i2, i2+lineCount
			opcodes = append(opcodes, Opcode{
				Tag: "delete",
				I1:  i1,
				I2:  i2,
				J1:  j1,
				J2:  j1, // no new lines
			})

		case diffmatchpatch.DiffInsert:
			j1, j2 = j2, j2+lineCount
			opcodes = append(opcodes, Opcode{
				Tag: "insert",
				I1:  i1,
				I2:  i1, // no old lines
				J1:  j1,
				J2:  j2,
			})
		}
	}

	// Merge consecutive delete+insert into replace
	return mergeReplaces(opcodes)
}

func mergeReplaces(opcodes []Opcode) []Opcode {
	if len(opcodes) == 0 {
		return opcodes
	}

	var result []Opcode
	i := 0

	for i < len(opcodes) {
		op := opcodes[i]

		// Check if current is delete and next is insert (= replace)
		if i+1 < len(opcodes) &&
			op.Tag == "delete" &&
			opcodes[i+1].Tag == "insert" {

			// Merge into replace
			next := opcodes[i+1]
			result = append(result, Opcode{
				Tag: "replace",
				I1:  op.I1,
				I2:  op.I2,
				J1:  next.J1,
				J2:  next.J2,
			})
			i += 2
		} else {
			result = append(result, op)
			i++
		}
	}

	return result
}
