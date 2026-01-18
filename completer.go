package console

import (
	"strings"

	"github.com/carapace-sh/carapace"
	"github.com/carapace-sh/carapace/pkg/style"
	completer "github.com/carapace-sh/carapace/pkg/x"
	"github.com/reeflective/readline"

	"github.com/reeflective/console/internal/completion"
	"github.com/reeflective/console/internal/line"
)

const (
	// defaultMaxHighlightRunes caps how many runes we feed to the syntax highlighter.
	// Large inputs force readline to repaint the whole line on every keystroke;
	// past this limit we skip highlighting to keep typing responsive (especially
	// on high-latency SSH links). The value is intentionally generous so typical
	// commands still get colored.
	defaultMaxHighlightRunes = 2048
	envMaxHighlightRunes     = "CONSOLE_MAX_HIGHLIGHT_RUNES"
)

func (c *Console) complete(input []rune, pos int) readline.Completions {
	menu := c.activeMenu()

	// Ensure the carapace library is called so that the function
	// completer.Complete() variable is correctly initialized before use.
	carapace.Gen(menu.Command)

	// Split the line as shell words, only using
	// what the right buffer (up to the cursor)
	args, prefixComp, prefixLine := completion.SplitArgs(input, pos)

	// Prepare arguments for the carapace completer
	// (we currently need those two dummies for avoiding a panic).
	args = append([]string{c.name, "_carapace"}, args...)

	// Call the completer with our current command context.
	completions, err := completer.Complete(menu.Command, args...)

	// The completions are never nil: fill out our own object
	// with everything it contains, regardless of errors.
	raw := make([]readline.Completion, len(completions.Values))

	for idx, val := range completions.Values {
		raw[idx] = readline.Completion{
			Value:       line.UnescapeValue(prefixComp, prefixLine, val.Value),
			Display:     val.Display,
			Description: val.Description,
			Style:       style.SGR(val.Style),
			Tag:         val.Tag,
		}

		if !completions.Nospace.Matches(val.Value) {
			raw[idx].Value = val.Value + " "
		}

		// Remove short/long flags grouping
		// join to single tag group for classic zsh side-by-side view
		switch val.Tag {
		case "shorthand flags", "longhand flags":
			raw[idx].Tag = "flags"
		}
	}

	// Assign both completions and command/flags/args usage strings.
	comps := readline.CompleteRaw(raw)
	comps = comps.Usage("%s", completions.Usage)
	comps = c.justifyCommandComps(comps)

	// If any errors arose from the completion call itself.
	if err != nil {
		comps = readline.CompleteMessage("failed to load config: " + err.Error())
	}

	// Completion status/errors
	for _, msg := range completions.Messages.Get() {
		comps = comps.Merge(readline.CompleteMessage(msg))
	}

	// Suffix matchers for the completions if any.
	suffixes, err := completions.Nospace.MarshalJSON()
	if len(suffixes) > 0 && err == nil {
		comps = comps.NoSpace([]rune(string(suffixes))...)
	}

	// If we have a quote/escape sequence unaccounted
	// for in our completions, add it to all of them.
	comps = comps.Prefix(prefixComp)
	comps.PREFIX = prefixLine

	// Finally, reset our command tree for the next call.
	completer.ClearStorage()
	menu.resetPreRun()
	menu.hideFilteredCommands(menu.Command)

	return comps
}

// justifyCommandComps justifies the descriptions for all commands in all groups
// to the same level, for prettiness. Also, removes any coloring from them, as currently,
// the carapace engine does add coloring to each group, and we don't want this.
func (c *Console) justifyCommandComps(comps readline.Completions) readline.Completions {
	justified := []string{}

	comps.EachValue(func(comp readline.Completion) readline.Completion {
		if !strings.HasSuffix(comp.Tag, "commands") {
			return comp
		}

		justified = append(justified, comp.Tag)
		comp.Style = "" // Remove command coloring

		return comp
	})

	if len(justified) > 0 {
		return comps.JustifyDescriptions(justified...)
	}

	return comps
}

// highlightSyntax - Entrypoint to all input syntax highlighting in the Wiregost console.
func (c *Console) highlightSyntax(input []rune) string {
	if len(input) == 0 {
		return ""
	}

	// Avoid expensive parsing and ANSI rewriting on very long lines.
	if c.MaxHighlightRunes > 0 && len(input) > c.MaxHighlightRunes {
		return string(input)
	}

	// Split the line as shellwords
	args, unprocessed, err := line.Split(string(input), true)
	if err != nil {
		args = append(args, unprocessed)
	}

	if len(args) == 0 {
		return string(input)
	}

	done := make([]string, 0, len(args)) // Processed words
	remain := args                       // Words to process
	changed := false

	// Highlight the root command when found (fast path, no cobra.Find walk).
	done, remain, changed = line.HighlightCommand(done, args, c.activeMenu().Command, c.cmdHighlight)

	// Highlight command flags
	var flagChanged bool
	done, remain, flagChanged = line.HighlightCommandFlags(done, remain, c.flagHighlight)
	changed = changed || flagChanged

	// Done with everything, add remaining, non-processed words
	if len(remain) > 0 {
		done = append(done, remain...)
	}

	// If we didn't apply any styling, avoid rebuilding the string.
	if !changed {
		return string(input)
	}

	// Join all words.
	highlighted := strings.Join(done, "")

	return highlighted
}
