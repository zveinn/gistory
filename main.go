package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type HistoryApp struct {
	app              *tview.Application
	inputField       *tview.InputField
	list             *tview.List
	header           *tview.Flex
	countView        *tview.TextView
	titleView        *tview.TextView
	combinedView     *tview.TextView
	mainContent      *tview.Flex
	history          []string
	bookmarks        []string
	bookmarkSet      map[string]bool
	showingBookmarks bool
	combinedCommands []string
	combinedChanged  bool
	filtered         []string
	searchQuery      string
}

func main() {
	bookmarksFlag := flag.Bool("bookmarks", false, "start directly in the bookmarks list")
	flag.Parse()

	commands, err := loadCommands()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading commands: %v\n", err)
		os.Exit(1)
	}

	if len(commands) == 0 {
		fmt.Fprintf(os.Stderr, "No commands found\n")
		os.Exit(1)
	}

	bookmarks, err := loadBookmarks()
	if err != nil {
		bookmarks = []string{}
	}

	ha := &HistoryApp{
		app:         tview.NewApplication(),
		history:     commands,
		bookmarks:   bookmarks,
		bookmarkSet: make(map[string]bool, len(bookmarks)),
		filtered:    commands,
	}

	for _, b := range bookmarks {
		ha.bookmarkSet[b] = true
	}

	if *bookmarksFlag {
		ha.showingBookmarks = true
		ha.filtered = append([]string(nil), ha.bookmarks...)
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

func readCache(path string) ([]string, error) {
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

func writeCache(path string, commands []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, cmd := range commands {
		if _, err := fmt.Fprintln(writer, cmd); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func loadCommands() ([]string, error) {
	home := os.Getenv("HOME")
	historyPath := filepath.Join(home, ".bash_history")
	cacheDir := filepath.Join(home, ".gistory")
	cachePath := filepath.Join(cacheDir, "commands")

	// Ensure the cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}

	bashHistory, err := readHistory(historyPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading bash history: %w", err)
	}

	cacheCommands, err := readCache(cachePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading cache: %w", err)
	}

	// Load both into a map to remove duplicates.
	// Prefer most recent commands from bash history.
	seen := make(map[string]bool)
	result := make([]string, 0, len(bashHistory)+len(cacheCommands))

	// Add commands from bash history (most recent first)
	for i := len(bashHistory) - 1; i >= 0; i-- {
		cmd := bashHistory[i]
		if !seen[cmd] {
			seen[cmd] = true
			result = append(result, cmd)
		}
	}

	// Add any unique commands from the cache
	for _, cmd := range cacheCommands {
		if !seen[cmd] {
			seen[cmd] = true
			result = append(result, cmd)
		}
	}

	// Rewrite the cache with the new de-duped list
	if err := writeCache(cachePath, result); err != nil {
		return nil, fmt.Errorf("writing cache: %w", err)
	}

	return result, nil
}

func loadBookmarks() ([]string, error) {
	home := os.Getenv("HOME")
	path := filepath.Join(home, ".gistory", "bookmarks")
	bookmarks, err := readCache(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	return bookmarks, nil
}

func saveBookmarks(bookmarks []string) error {
	home := os.Getenv("HOME")
	path := filepath.Join(home, ".gistory", "bookmarks")
	return writeCache(path, bookmarks)
}

func (ha *HistoryApp) buildUI() {
	accent := tcell.NewRGBColor(96, 165, 250)

	inputBox := tview.NewInputField().
		SetLabel(" [#60a5fa::b]$[-] ").
		SetFieldWidth(0).
		SetChangedFunc(func(text string) {
			ha.searchQuery = text
			ha.filterHistory(text)
		})

	inputBox.SetLabelColor(accent).
		SetFieldTextColor(tcell.NewRGBColor(255, 255, 255)).
		SetFieldBackgroundColor(tcell.ColorDefault)

	ha.inputField = inputBox

	// Create list with custom styling
	ha.list = tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true)

	ha.list.SetMainTextColor(tcell.NewRGBColor(224, 224, 230)).
		SetSelectedTextColor(tcell.ColorWhite).
		SetSelectedBackgroundColor(tcell.NewRGBColor(38, 55, 85)).
		SetShortcutColor(accent)

	ha.list.SetBackgroundColor(tcell.ColorDefault)

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
		case tcell.KeyCtrlB:
			ha.showingBookmarks = !ha.showingBookmarks
			ha.filterHistory(ha.searchQuery)
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			// normal backspace (or ctrl variants) - ctrl+backspace is handled at app level
			// to prioritize removing from combined over editing the search text
			return event
		}
		return event
	})

	ha.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			ha.app.Stop()
			return nil
		case tcell.KeyEnter:
			if len(ha.combinedCommands) > 0 {
				combined := strings.Join(ha.combinedCommands, " ; ")
				ha.app.Stop()
				fmt.Print(combined)
				return nil
			}
			// fall through to selected func for normal case
		case tcell.KeyLeft:
			if idx := ha.list.GetCurrentItem(); idx >= 0 && idx < len(ha.filtered) {
				cmd := ha.filtered[idx]
				ha.combinedCommands = append(ha.combinedCommands, cmd)
				ha.combinedChanged = true
				// ForceDraw is explicitly safe to call during direct event handling
				// (per tview docs). It runs BeforeDraw (the safe hook for structural
				// layout changes like conditionally adding the combined bar) then the
				// actual draw. This prevents freezing/reentrancy.
				ha.app.ForceDraw()
			}
			return nil
		case tcell.KeyRight:
			idx := ha.list.GetCurrentItem()
			if idx >= 0 && idx < len(ha.filtered) {
				cmd := ha.filtered[idx]
				ha.toggleBookmark(cmd)
				ha.filterHistory(ha.searchQuery)

				// Try to keep the cursor on the same command (or same position if it was removed)
				if len(ha.filtered) > 0 {
					newIdx := idx
					for i, c := range ha.filtered {
						if c == cmd {
							newIdx = i
							break
						}
					}
					if newIdx >= len(ha.filtered) {
						newIdx = len(ha.filtered) - 1
					}
					ha.list.SetCurrentItem(newIdx)
				}
			}
			return nil
		case tcell.KeyCtrlB:
			ha.showingBookmarks = !ha.showingBookmarks
			ha.filterHistory(ha.searchQuery)
			return nil
		case tcell.KeyRune:
			currentText := ha.inputField.GetText()
			ha.inputField.SetText(currentText + string(event.Rune()))
			ha.app.SetFocus(ha.inputField)
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			// normal backspace: send to input field
			// (Ctrl+Backspace for combined is handled by app-level input capture)
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

	// Top bar: left title + right live result count (no border)
	header := tview.NewFlex().SetDirection(tview.FlexColumn)
	header.SetBackgroundColor(tcell.NewRGBColor(30, 38, 52))
	titleView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(" [#60a5fa::b]$[-] [#60a5fa::b]gistory[-]")
	countView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)

	ha.titleView = titleView

	header.AddItem(titleView, 0, 0, false)
	header.AddItem(nil, 0, 1, false)
	header.AddItem(countView, 16, 0, false)

	ha.header = header

	ha.combinedView = tview.NewTextView().
		SetDynamicColors(true)
	ha.combinedView.SetBackgroundColor(tcell.NewRGBColor(35, 40, 48))

	ha.mainContent = tview.NewFlex().SetDirection(tview.FlexRow)
	ha.rebuildMainContent()

	ha.countView = countView

	// Populate list + set initial count in top bar
	ha.updateList()

	// Small side margins
	root := tview.NewFlex().
		AddItem(nil, 2, 0, false).
		AddItem(ha.mainContent, 0, 1, true).
		AddItem(nil, 2, 0, false)

	ha.updateTitle()

	ha.app.SetRoot(root, true)
	ha.app.SetFocus(ha.inputField)

	// Global input capture to handle Ctrl+Backspace for removing combined commands,
	// even when the input search bar has focus. This prevents the search bar from
	// consuming the key for editing the query text.
	ha.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlH ||
			((event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2) &&
				event.Modifiers()&tcell.ModCtrl != 0) {
			if len(ha.combinedCommands) > 0 {
				ha.combinedCommands = ha.combinedCommands[:len(ha.combinedCommands)-1]
				ha.combinedChanged = true
				ha.app.ForceDraw()
				return nil
			}
		}
		return event
	})

	// Use BeforeDraw to safely perform structural layout changes (e.g. adding/removing
	// the combined bar) right before rendering. This avoids reentrancy issues when
	// triggered from InputCapture handlers.
	ha.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		if ha.combinedChanged {
			ha.rebuildMainContent()
			ha.combinedChanged = false
		}
		return false
	})
}

func (ha *HistoryApp) filterHistory(query string) {
	source := ha.history
	if ha.showingBookmarks {
		source = ha.bookmarks
	}
	ha.filtered = filterAndSortCommands(source, query)
	ha.updateList()
	ha.updateTitle()
}

func (ha *HistoryApp) updateTitle() {
	if ha.titleView == nil {
		return
	}
	title := " [#60a5fa::b]$[-] [#60a5fa::b]gistory[-]"
	if ha.showingBookmarks {
		title = " [#60a5fa::b]$[-] [#60a5fa::b]gistory[-] [#60a5fa::b][bookmarks][-]"
	}
	ha.titleView.SetText(title)
}

func (ha *HistoryApp) rebuildMainContent() {
	if ha.mainContent == nil {
		return
	}

	ha.mainContent.Clear()
	ha.mainContent.AddItem(ha.header, 1, 0, false)
	ha.mainContent.AddItem(ha.inputField, 1, 0, false)

	if len(ha.combinedCommands) > 0 {
		combinedText := strings.Join(ha.combinedCommands, " ; ")
		// White font, no dimming. Background is already set on the view.
		ha.combinedView.SetText("[white]" + combinedText + "[-]")
		ha.mainContent.AddItem(ha.combinedView, 1, 0, false)
	}

	ha.mainContent.AddItem(ha.list, 0, 1, false)
}

func (ha *HistoryApp) toggleBookmark(cmd string) {
	if ha.bookmarkSet == nil {
		ha.bookmarkSet = make(map[string]bool)
	}

	if ha.bookmarkSet[cmd] {
		delete(ha.bookmarkSet, cmd)
		// remove from slice while preserving order
		newBookmarks := make([]string, 0, len(ha.bookmarks))
		for _, b := range ha.bookmarks {
			if b != cmd {
				newBookmarks = append(newBookmarks, b)
			}
		}
		ha.bookmarks = newBookmarks
	} else {
		ha.bookmarkSet[cmd] = true
		ha.bookmarks = append(ha.bookmarks, cmd)
	}

	_ = saveBookmarks(ha.bookmarks)
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

		prefix := "  "
		if ha.bookmarkSet != nil && ha.bookmarkSet[cmd] {
			prefix = "[yellow]★[-] "
		}

		if ha.searchQuery != "" {
			displayCmd = highlightMatches(cmd, ha.searchQuery)
		}

		if len(cmd) > 200 {
			displayCmd = displayCmd[:200] + "[gray]…[-]"
		}

		ha.list.AddItem(prefix+displayCmd, "", 0, nil)
	}

	// Update top bar result count (purely visual)
	if ha.countView != nil {
		count := len(ha.filtered)
		ha.countView.SetText(fmt.Sprintf("[gray]%d results[-] ", count))
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
			result.WriteString("[#60a5fa::bu]")
			result.WriteByte(text[i])
			result.WriteString("[-]")
			patternIdx++
		} else {
			result.WriteByte(text[i])
		}
	}

	return result.String()
}

func (ha *HistoryApp) selectCommand(index int) {
	if len(ha.combinedCommands) > 0 {
		combined := strings.Join(ha.combinedCommands, " ; ")
		ha.app.Stop()
		fmt.Print(combined)
		return
	}
	if index >= 0 && index < len(ha.filtered) {
		ha.app.Stop()
		fmt.Print(ha.filtered[index])
	}
}
