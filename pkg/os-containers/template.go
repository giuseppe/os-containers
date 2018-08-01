package oscontainers

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
)

const (
	TemplateStateNone = iota
	TemplateStateInVariable
	TemplateStateInVariableBrace
)

func isValidVariableSymbol(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_'
}

func TemplateReplace(r *bufio.Reader, w io.Writer, values map[string]string) error {
	var currentVariable string
	state := TemplateStateNone
	for {
		rune, _, err := r.ReadRune()
		if err != nil && err != io.EOF {
			return err
		}

		if err == io.EOF {
			if state != TemplateStateNone {
				return fmt.Errorf("invalid template file")
			}
			return nil
		}

		switch state {
		case TemplateStateNone:
			if rune == '$' {
				nextRune, _, err := r.ReadRune()
				if err != nil {
					if err == io.EOF {
						return fmt.Errorf("invalid template file")
					}
					return err
				}
				if nextRune != '$' {
					currentVariable = ""
					if nextRune == '{' {
						state = TemplateStateInVariableBrace
					} else if isValidVariableSymbol(nextRune) {
						state = TemplateStateInVariable
						currentVariable = string(nextRune)
					} else {
						return fmt.Errorf("invalid template variable")
					}
					continue
				}
			}
			_, err = w.Write([]byte(string(rune)))
			if err != nil {
				return err
			}
			break
		case TemplateStateInVariable:
			if isValidVariableSymbol(rune) {
				currentVariable = currentVariable + string(rune)
				continue
			} else {
				v, found := values[currentVariable]
				if !found {
					return fmt.Errorf("invalid template, cannot find variable %s", currentVariable)
				}
				_, err = w.Write([]byte(v))
				if err != nil {
					return err
				}
				_, err = w.Write([]byte(string(rune)))
				if err != nil {
					return err
				}
				state = TemplateStateNone
				continue
			}
			break
		case TemplateStateInVariableBrace:
			if rune == '}' {
				v, found := values[currentVariable]
				if !found {
					return fmt.Errorf("invalid template, cannot find variable %s", currentVariable)
				}
				_, err = w.Write([]byte(v))
				if err != nil {
					return err
				}
				state = TemplateStateNone
			} else {
				currentVariable = currentVariable + string(rune)
			}
			break
		}
	}
}

func TemplateWithDefaultGenerate(in, out, def string, values map[string]string) error {
	var reader *bufio.Reader

	inFile, err := os.Open(in)
	if err == nil {
		defer inFile.Close()
		reader = bufio.NewReader(inFile)
	} else {
		if !os.IsNotExist(err) || def == "" {
			return err
		}
		reader = bufio.NewReader(bytes.NewReader([]byte(def)))
	}

	outFile, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE, 0700)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return TemplateReplace(reader, outFile, values)
}

func TemplateReplaceMemory(in string, values map[string]string) (string, error) {
	reader := bufio.NewReader(bytes.NewReader([]byte(in)))
	var writer bytes.Buffer

	err := TemplateReplace(reader, &writer, values)
	if err != nil {
		return "", err
	}

	return writer.String(), nil
}
