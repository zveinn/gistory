package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type HistoryApp struct {
	app         *tview.Application
	inputField  *tview.InputField
	list        *tview.List
	history     []string
	filtered    []string
	searchQuery string
}

func main() {
	historyPath := filepath.Join(os.Getenv("HOME"), ".bash_history")
	history, err := readHistory(historyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading history: %v\n", err)
		os.Exit(1)
	}

	if len(history) == 0 {
		fmt.Fprintf(os.Stderr, "No history found\n")
		os.Exit(1)
	}

	// Deduplicate and reverse (most recent first)
	history = deduplicateHistory(history)

	ha := &HistoryApp{
		app:      tview.NewApplication(),
		history:  history,
		filtered: history,
	}

	ha.buildUI()

	if err := ha.app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		os.Exit(1)
	}
}

func readHistory(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

func deduplicateHistory(history []string) []string {
	// Keep most recent occurrence of each command
	seen := make(map[string]bool)
	result := make([]string, 0)

	// Process from end to beginning (most recent first)
	for i := len(history) - 1; i >= 0; i-- {
		if !seen[history[i]] {
			seen[history[i]] = true
			result = append(result, history[i])
		}
	}

	return result
}

func (ha *HistoryApp) buildUI() {
	inputBox := tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0).
		SetChangedFunc(func(text string) {
			ha.searchQuery = text
			ha.filterHistory(text)
		})

	inputBox.SetLabelColor(tcell.NewRGBColor(150, 100, 200)).
		SetFieldTextColor(tcell.NewRGBColor(255, 255, 255)).
		SetFieldBackgroundColor(tcell.ColorDefault)

	ha.inputField = inputBox

	// Create list with custom styling
	ha.list = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true)

	ha.list.SetMainTextColor(tcell.ColorWhite).
		SetSelectedTextColor(tcell.ColorWhite).
		SetSelectedBackgroundColor(tcell.NewRGBColor(30, 30, 30)).
		SetShortcutColor(tcell.NewRGBColor(150, 100, 200))

	ha.list.SetBackgroundColor(tcell.ColorDefault)

	ha.updateList()

	ha.inputField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			ha.app.Stop()
			return nil
		case tcell.KeyDown, tcell.KeyCtrlN:
			if len(ha.filtered) > 1 {
				ha.list.SetCurrentItem(1)
			}
			ha.app.SetFocus(ha.list)
			return nil
		case tcell.KeyUp, tcell.KeyCtrlP:
			ha.app.SetFocus(ha.list)
			return nil
		case tcell.KeyEnter:
			if len(ha.filtered) > 0 {
				ha.selectCommand(0)
			}
			return nil
		}
		return event
	})

	ha.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			ha.app.Stop()
			return nil
		case tcell.KeyRune:
			currentText := ha.inputField.GetText()
			ha.inputField.SetText(currentText + string(event.Rune()))
			ha.app.SetFocus(ha.inputField)
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			currentText := ha.inputField.GetText()
			if len(currentText) > 0 {
				ha.inputField.SetText(currentText[:len(currentText)-1])
			}
			ha.app.SetFocus(ha.inputField)
			return nil
		}
		return event
	})

	ha.list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		ha.selectCommand(index)
	})

	listContainer := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ha.list, 0, 1, false)

	listFrame := tview.NewFrame(listContainer).
		SetBorders(0, 0, 1, 0, 0, 0)

	listWithBorder := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(listFrame, 0, 1, false)

	mainContent := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ha.inputField, 1, 0, true).
		AddItem(listWithBorder, 0, 1, false)

	flex := tview.NewFlex().
		AddItem(nil, 1, 0, false).
		AddItem(mainContent, 0, 1, true).
		AddItem(nil, 1, 0, false)

	ha.app.SetRoot(flex, true)
	ha.app.SetFocus(ha.inputField)
}

func (ha *HistoryApp) filterHistory(query string) {
	ha.filtered = filterAndSortCommands(ha.history, query)
	ha.updateList()
}

func filterAndSortCommands(history []string, query string) []string {
	if query == "" {
		return history
	}

	prefixMatches := make([]string, 0)
	substringMatches := make([]string, 0)
	fuzzyMatches := make([]string, 0)

	lowerQuery := strings.ToLower(query)

	for _, cmd := range history {
		lowerCmd := strings.ToLower(cmd)

		if strings.HasPrefix(lowerCmd, lowerQuery) {
			prefixMatches = append(prefixMatches, cmd)
		} else if strings.Contains(lowerCmd, lowerQuery) {
			substringMatches = append(substringMatches, cmd)
		} else if fuzzyMatch(lowerCmd, lowerQuery) {
			fuzzyMatches = append(fuzzyMatches, cmd)
		}
	}

	result := make([]string, 0, len(prefixMatches)+len(substringMatches)+len(fuzzyMatches))
	result = append(result, prefixMatches...)
	result = append(result, substringMatches...)
	result = append(result, fuzzyMatches...)

	return result
}

func fuzzyMatch(text, pattern string) bool {
	patternIdx := 0
	for i := 0; i < len(text) && patternIdx < len(pattern); i++ {
		if text[i] == pattern[patternIdx] {
			patternIdx++
		}
	}
	return patternIdx == len(pattern)
}

func (ha *HistoryApp) updateList() {
	ha.list.Clear()

	maxItems := min(len(ha.filtered), 100)

	for i := range maxItems {
		cmd := ha.filtered[i]
		displayCmd := cmd

		if ha.searchQuery != "" {
			displayCmd = highlightMatches(cmd, ha.searchQuery)
		}

		if len(cmd) > 200 {
			displayCmd = displayCmd[:200] + "[grey]...[white]"
		}

		ha.list.AddItem("  "+displayCmd, "", 0, nil)
	}
}

func highlightMatches(text, pattern string) string {
	if pattern == "" {
		return text
	}

	lowerText := strings.ToLower(text)
	lowerPattern := strings.ToLower(pattern)

	var result strings.Builder
	patternIdx := 0

	for i := 0; i < len(text); i++ {
		if patternIdx < len(lowerPattern) && lowerText[i] == lowerPattern[patternIdx] {
			result.WriteString("[#9664c8::b]")
			result.WriteByte(text[i])
			result.WriteString("[white::-]")
			patternIdx++
		} else {
			result.WriteByte(text[i])
		}
	}

	return result.String()
}

func (ha *HistoryApp) selectCommand(index int) {
	if index >= 0 && index < len(ha.filtered) {
		ha.app.Stop()
		fmt.Print(ha.filtered[index])
	}
}
