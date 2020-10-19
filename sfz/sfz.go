package sfz

import (
	"io/ioutil"
	"path"
	"strings"
)

type SfzValuePair struct {
	Name  string
	Value string
}

type SfzSection struct {
	Name       string
	ValuePairs []SfzValuePair
}

type SfzFile struct {
	Sections []*SfzSection
}

type sfzParseStackFrame struct {
	source   []rune
	parsePos int
	cwd      string
}

type sfzParseContext struct {
	definitions  map[string]string
	currentLabel string
	currentValue string
	defaultPath  string
}

func createStackFrame(filename string) (*sfzParseStackFrame, error) {
	content, err := ioutil.ReadFile(filename)

	if err != nil {
		return nil, err
	}

	return &sfzParseStackFrame{
		[]rune(string(content)),
		0,
		path.Dir(filename),
	}, nil
}

func isSeparator(char rune) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '='
}

func skipComment(frame *sfzParseStackFrame) bool {
	var curr = frame.parsePos

	if curr+2 > len(frame.source) || frame.source[curr] != '/' || frame.source[curr+1] != '/' {
		return false
	}

	for curr < len(frame.source) && frame.source[curr] != '\n' {
		curr = curr + 1
	}

	if frame.parsePos != curr {
		frame.parsePos = curr
		return true
	} else {
		return false
	}
}

func nextToken(context *sfzParseContext, frame *sfzParseStackFrame) (token string, prevSep string, next rune) {
	var start = frame.parsePos
	var curr = frame.parsePos

	var skippingWhitespace = true

	for skippingWhitespace {
		for curr < len(frame.source) && isSeparator(frame.source[curr]) {
			curr = curr + 1
		}

		skippingWhitespace = skipComment(frame)
	}

	var tokenStart = curr

	for curr < len(frame.source) && !isSeparator(frame.source[curr]) {
		curr = curr + 1
	}

	var tokenEnd = curr
	var eqCheck = curr

	for eqCheck < len(frame.source) && isSeparator(frame.source[eqCheck]) {
		if frame.source[eqCheck] == '=' {
			curr = eqCheck
			break
		}

		eqCheck = eqCheck + 1
	}

	frame.parsePos = curr

	if curr < len(frame.source) {
		next = frame.source[curr]
	} else {
		next = '\000'
	}

	token = string(frame.source[tokenStart:tokenEnd])

	tokenRename, exists := context.definitions[token]

	if exists {
		token = tokenRename
	}

	return token, string(frame.source[start:tokenStart]), next
}

func appendSection(target *SfzFile, name string) {
	target.Sections = append(target.Sections, &SfzSection{
		name,
		nil,
	})
}

func getLastSection(target *SfzFile) *SfzSection {
	if len(target.Sections) == 0 {
		appendSection(target, "")
	}

	return target.Sections[len(target.Sections)-1]
}

func checkFinishPair(target *SfzFile, context *sfzParseContext, frame *sfzParseStackFrame) {
	if context.currentLabel != "" {
		var section = getLastSection(target)

		if context.currentLabel == "sample" {
			var cleanedPath = path.Clean(strings.Replace(context.currentValue, "\\", "/", -1))
			if context.defaultPath == "" {
				context.currentValue = path.Join(frame.cwd, cleanedPath)
			} else {
				context.currentValue = path.Join(context.defaultPath, cleanedPath)
			}
		} else if context.currentLabel == "default_path" {
			context.defaultPath = path.Join(frame.cwd, path.Clean(strings.Replace(context.currentValue, "\\", "/", -1)))
		}

		section.ValuePairs = append(section.ValuePairs, SfzValuePair{
			context.currentLabel,
			context.currentValue,
		})

		context.currentLabel = ""
		context.currentValue = ""
	}
}

func parseInto(target *SfzFile, context *sfzParseContext, frame *sfzParseStackFrame) error {
	var hasNext = true

	for hasNext {
		current, prevSep, nextChar := nextToken(context, frame)

		if current == "" {
			hasNext = false
		} else if current == "#define" {
			name, _, _ := nextToken(context, frame)
			value, _, _ := nextToken(context, frame)

			context.definitions[name] = value
		} else if current == "#include" {
			includePath, _, _ := nextToken(context, frame)

			for includePath[len(includePath)-1] != '"' {
				pathSegment, whitespace, _ := nextToken(context, frame)
				includePath = includePath + whitespace + pathSegment
			}

			var absolutePath = path.Join(frame.cwd, includePath)

			nextStackFrame, err := createStackFrame(absolutePath)

			if err != nil {
				return err
			}

			parseInto(target, context, nextStackFrame)
		} else if current[0] == '<' && current[len(current)-1] == '>' {
			checkFinishPair(target, context, frame)
			appendSection(target, current)
		} else if nextChar == '=' {
			checkFinishPair(target, context, frame)
			context.currentLabel = current
		} else if context.currentLabel != "" {
			if context.currentValue != "" {
				context.currentValue = context.currentValue + prevSep + current
			} else {
				context.currentValue = current
			}
		}
	}

	checkFinishPair(target, context, frame)

	return nil
}

func ParseSfz(filename string) (*SfzFile, error) {
	stackFrame, err := createStackFrame(filename)

	if err != nil {
		return nil, err
	}

	var result SfzFile
	var context = sfzParseContext{
		nil,
		"",
		"",
		"",
	}

	err = parseInto(&result, &context, stackFrame)

	if err != nil {
		return nil, err
	}

	return &result, nil
}