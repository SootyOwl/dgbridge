package lib

import (
	"dgbridge/src/ext"
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Added regex to strip ANSI color codes
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type (
	Rules struct {
		DiscordToSubprocess []Rule `validate:"required"`
		SubprocessToDiscord []Rule `validate:"required"`
	}
	Rule struct {
		Match    ext.Regexp `validate:"required"`
		Template string     `validate:"required"`
	}
)

type (
	Props struct {
		Author Author `validate:"required"`
	}
	Author struct {
		Username      string `validate:"required"`
		Nickname      string // Nickname might not be set
		GlobalName    string // GlobalName might not be set
		Discriminator string `validate:"required"`
		AccentColor   int    `validate:"required"`
	}
)


// LoadRules loads a set of rules from a JSON file.
func LoadRules(path string) (*Rules, error) {
	fileContents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rules Rules
	err = json.Unmarshal(fileContents, &rules)
	if err != nil {
		return nil, err
	}
	return &rules, err
}

// ApplyRules applies rules to a string.
// If props are provided, a matching template will be built using those props.
func ApplyRules(rules []Rule, props *Props, input string) string {
	for _, rule := range rules {
		result := ApplyRule(rule, props, input)
		if result != "" {
			// Strip ANSI color codes from the line before sending it to Discord
			// This is necessary to avoid sending raw ANSI codes to Discord, which are
			// ugly, but still allows the subprocess to use colors and the rules to match
			// using ANSI codes.
			result = ansiRegex.ReplaceAllString(result, "")
			return result
		}
	}
	return ""
}

// ApplyRule applies a rule to a given input string if it matches.
//
// Parameters:
// props: If passed, the Rule's template is built with the given Props.
func ApplyRule(rule Rule, props *Props, input string) string {
	// Remove newlines from input and replace them with spaces
	input = strings.ReplaceAll(input, "\n", " ")

	if rule.Match.MatchString(input) {
		if props == nil {
			return rule.Match.ReplaceAllString(input, rule.Template)
		}
		return rule.Match.ReplaceAllString(input, buildTemplate(rule.Template, *props))
	}
	return ""
}

// Builds a rule template for Discord -> Process communication.
// It replaces all special combinations in the template with their corresponding properties.
//
// Example:
//   - ^U turns into Username
//   - ^T turns into Discriminator
//   - ^C turns into RoleColor/AccentColor
//   - ^N turns into Nickname (or Username if Nickname is not set)
//
// Returns template with Props applied.
func buildTemplate(template string, props Props) string {
	var result []rune
	runes := []rune(template)
	for i := 0; i < len(runes); i++ {
		currentRune := runes[i]
		if currentRune == '^' && i+1 < len(template) {
			switch template[i+1] {
			case '^':
				// This is an escaped ^
				result = append(result, '^')
				i++
				continue
			case 'U':
				if props.Author.GlobalName != "" {
					result = append(result, []rune(props.Author.GlobalName)...)
				} else {
					result = append(result, []rune(props.Author.Username)...)
				}
				i++
				continue
			case 'T':
				result = append(result, []rune(props.Author.Discriminator)...)
				i++
				continue
			case 'C':
				result = append(result, []rune(strconv.FormatInt(int64(props.Author.AccentColor), 16))...)
				i++
				continue
			case 'N':
				if props.Author.Nickname != "" {
					result = append(result, []rune(props.Author.Nickname)...)
				} else if props.Author.GlobalName != "" {
					result = append(result, []rune(props.Author.GlobalName)...)
				} else {
					result = append(result, []rune(props.Author.Username)...)
				}
				i++
				continue
			}
		}
		result = append(result, currentRune)
	}
	return string(result)
}
