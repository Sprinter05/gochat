package ui

import (
	"errors"
	"fmt"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const Logo string = `
                   _           _   
                  | |         | |  
   __ _  ___   ___| |__   __ _| |_ 
  / _  |/ _ \ / __| '_ \ / _  | __|
 | (_| | (_) | (__| | | | (_| | |_ 
  \__  |\___/ \___|_| |_|\____|\__|
   __/ |                           
  |___/   

`

const (
	tuiVersion      float32 = 0.4       // Current client TUI version
	selfSender      string  = "You"     // Self sender of a message
	systemBuffer    string  = "System"  // System buffer name
	debugBuffer     string  = "Debug"   // Buffer where packets will be shown
	defaultBuffer   string  = "Default" // Default server system buffer
	localServer     string  = "Local"   // Local server name
	defaultLabel    string  = " > "     // Default prompt
	defaultUserlist string  = "(Empty)" // Default userlist text
	inputSize       int     = 4         // Default size of the text input bar (fixed)
	errorSize       int     = 1         // Default size of the error bar (fixed)
	notifSize       int     = 2         // Default size of the notif bar (fixed)
	textSize        int     = 30        // Default size of the text window
	errorMessage    uint    = 3         // Amount of seconds the error text shows up
	asciiNumbers    int     = 0x30      // Start of ASCII for number 1
	asciiLowercase  int     = 0x61      // Start of ASCII for lowercase a
	maxBuffers      uint    = 35        // Maximum amount of allowed buffers in one server
	maxServers      uint    = 9         // Maximum amount of allowed servers
	cmdTimeout      uint    = 15        // Max seconds to wait for a command to finish
	msgDelay        uint    = 300       // Miliseconds between sending messages
	rootBuffer      uint    = 0         // Number of the root buffer
	textPage        string  = "Text"    // Name of the text page
	helpPage        string  = "Help"    // Name of the help page
)

var (
	ErrorSystemBuf        = errors.New("performing action on system buffer")          // performing action on system buffer
	ErrorLocalServer      = errors.New("performing action on local server")           // performing action on local server
	ErrorNoText           = errors.New("no text has been given")                      // no text has been given
	ErrorExists           = errors.New("item already exists")                         // item already exists
	ErrorNotFound         = errors.New("item does not exist")                         // item does not exist
	ErrorMaxBufs          = errors.New("maximum amount of buffers reached")           // maximum amount of buffers reached
	ErrorMaxServers       = errors.New("maximum amount of servers reached")           // maximum amount of servers reached
	ErrorNoBuffers        = errors.New("no buffers in server")                        // no buffers in server
	ErrorEmptyCmd         = errors.New("empty command given")                         // empty command given
	ErrorInvalidCmd       = errors.New("invalid command given")                       // invalid command given
	ErrorAlreadyOnline    = errors.New("connection is already established")           // connection is already established
	ErrorOffline          = errors.New("connection to the server is not established") // connection to the server is not established
	ErrorArguments        = errors.New("invalid number of arguments")                 // invalid number of arguments
	ErrorLoggedIn         = errors.New("you are already logged in")                   // you are already logged in
	ErrorNoRemoteUser     = errors.New("user is not requested")                       // user is not requested
	ErrorDisconnection    = errors.New("connection to the server has been lost")      // connection to the server has been lost
	ErrorNotLoggedIn      = errors.New("you are not logged in")                       // you are not logged in
	ErrorMessageSelf      = errors.New("cannot request to message yourself")          // cannot request to message yourself
	ErrorTypingTooFast    = errors.New("you are typing too fast")                     // you are typing too fast
	ErrorPasswordNotMatch = errors.New("passwords do not match")                      // passwords do not match
	ErrorInvalidArgument  = errors.New("provided argument is incorrect")              // provided argument is incorrect
	ErrorMessageFromSelf  = errors.New("received message from self")                  // received message from self
	ErrorInvalidAddress   = errors.New("address of server is not valid")              // address of server is not valid
)

// Identifies the areas where components are located.
type areas struct {
	main   *tview.Flex // main area composed of every component
	bottom *tview.Flex // bottom area composed of text, input and errors
	left   *tview.Flex // left area composed of buffers and servers
}

// Identifies the individual components of the TUI.
type components struct {
	buffers *tview.List // list of buffers
	servers *tview.List // list of servers

	pages *tview.Pages // switch between main window pages

	notifs *tview.TextView // shows notifications
	text   *tview.TextView // shows messages
	help   *tview.TextView
	errors *tview.TextView // shows TUI errors
	input  *tview.TextArea // input area to type

	users *tview.TextView // list of users
}

// Returns the default sizes for the layout
// in the parameters struct
func defaultParams() Parameters {
	return Parameters{
		Buflist: ComponentSize{
			Relative: true,
			Size:     2,
		},
		Userlist: ComponentSize{
			Relative: true,
			Size:     1,
		},
	}
}

// Creates all components and assigns them to each area.
func setupLayout() (areas, components) {
	comps := components{
		buffers: tview.NewList(),
		servers: tview.NewList(),
		pages:   tview.NewPages(),
		notifs:  tview.NewTextView(),
		text:    tview.NewTextView(),
		help:    tview.NewTextView(),
		errors:  tview.NewTextView(),
		input:   tview.NewTextArea(),
		users:   tview.NewTextView(),
	}

	comps.pages.AddPage(textPage, comps.text, true, false)
	comps.pages.AddPage(helpPage, comps.help, true, false)

	bottom := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(comps.notifs, 0, 0, false).
		AddItem(comps.pages, 0, textSize, false).
		AddItem(comps.errors, 0, 0, false).
		AddItem(comps.input, inputSize, 0, true)
	bottom.SetBackgroundColor(tcell.ColorDefault)

	left := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(comps.buffers, 0, 3, false).
		AddItem(comps.servers, 0, 1, false)
	left.SetBackgroundColor(tcell.ColorDefault)

	main := tview.NewFlex().
		AddItem(left, 0, 0, false).
		AddItem(bottom, 0, 6, true).
		AddItem(comps.users, 0, 0, false)
	main.SetBackgroundColor(tcell.ColorDefault)

	areas := areas{
		main:   main,
		bottom: bottom,
		left:   left,
	}

	return areas, comps
}

// Sets up the options for each individual component.
func setupStyle(t *TUI) {
	t.comp.text.
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetScrollable(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).
		SetTitle("Messages")

	t.comp.help.
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetScrollable(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).
		SetTitle("Help")

	t.comp.buffers.
		SetMainTextStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault)).
		SetShortcutStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault).
			Foreground(tcell.ColorYellow)).
		SetSelectedStyle(tcell.StyleDefault.Underline(true)).
		SetSelectedTextColor(tcell.ColorPurple).
		ShowSecondaryText(false).
		SetBorder(true).
		SetTitle("Buffers").
		SetBackgroundColor(tcell.ColorDefault)

	t.comp.users.
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(false).
		SetScrollable(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).
		SetTitle("Users")

	t.comp.servers.
		SetMainTextStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault)).
		SetSecondaryTextStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault).
			Foreground(tcell.ColorDarkGray)).
		SetShortcutStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault).
			Foreground(tcell.ColorYellow)).
		ShowSecondaryText(true).
		SetSelectedStyle(tcell.StyleDefault.Underline(true)).
		SetSelectedTextColor(tcell.ColorPurple).
		SetTitle("Servers").
		SetBorder(true).
		SetBackgroundColor(tcell.ColorDefault)

	t.comp.input.
		SetLabel(defaultLabel).
		SetMaxLength(spec.MaxArgSize).
		SetTextStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault)).
		SetPlaceholderStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault).
			Foreground(tcell.ColorGreen)).
		SetPlaceholder("Write here...").
		SetWrap(true).
		SetWordWrap(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true)

	t.comp.errors.
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(false)

	t.comp.notifs.
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(false)
}

// Sets up the handling functions for each component.
func setupHandlers(t *TUI) {
	// Runs after selecting a buffer
	t.comp.buffers.SetDoneFunc(func() {
		t.app.SetFocus(t.comp.input)
	})

	// Runs after selecting a server
	t.comp.servers.SetDoneFunc(func() {
		t.app.SetFocus(t.comp.input)
	})

	// Runs when selecting a buffer
	t.comp.buffers.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
		t.renderBuffer(s1)
		t.app.SetFocus(t.comp.input)
	})

	// Runs when selecting a server
	t.comp.servers.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
		t.renderServer(s1)
		t.app.SetFocus(t.comp.input)
	})

	// Keybinds for the buffer list
	t.comp.buffers.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlN: // New buffer
			if !t.status.blockCond() {
				newbufPopup(t)
			}
		case tcell.KeyCtrlW, tcell.KeyCtrlH: // Hide buffer
			if !t.status.blockCond() {
				t.hideBuffer(t.Buffer())
				t.app.SetFocus(t.comp.input)
			}
		case tcell.KeyCtrlX: // Remove buffer
			if !t.status.blockCond() {
				deleteBufWindow(t)
			}
		}
		return event
	})

	// Keybinds for the server list
	t.comp.servers.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlN: // New server
			if !t.status.blockCond() {
				newServerPopup(t)
			}
		case tcell.KeyCtrlW, tcell.KeyCtrlH: // Hide server
			if !t.status.blockCond() {
				t.hideServer(t.focus)
				t.app.SetFocus(t.comp.input)
			}
		case tcell.KeyCtrlX: // Remove server
			if !t.status.blockCond() {
				deleteServWindow(t)
			}
		}
		return event
	})

	// Forces a redraw when new text shows up
	t.comp.text.SetChangedFunc(func() {
		t.app.Draw()
	})

	// Forces a redraw when new text shows up
	t.comp.errors.SetChangedFunc(func() {
		t.app.Draw()
	})

	// Forces a redraw when new text shows up
	t.comp.users.SetChangedFunc(func() {
		t.app.Draw()
	})

	// Forces a redraw when new text shows up
	t.comp.notifs.SetChangedFunc(func() {
		t.app.Draw()
	})
}

// Sets up main input capture (run command, send text, newline).
func setupInput(t *TUI) {
	// Go back to the initial history
	t.comp.input.SetChangedFunc(func() {
		text := t.comp.input.GetText()
		if text == "" {
			t.next = 0
		}
	})

	// Text window keybinds
	t.comp.text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyESC: // Scroll to the bottom/top
			if event.Modifiers()&tcell.ModAlt == tcell.ModAlt ||
				event.Modifiers()&tcell.ModShift == tcell.ModShift {
				t.comp.text.ScrollToBeginning()
			} else {
				t.comp.text.ScrollToEnd()
			}
		}
		return event
	})

	// Input window keybinds
	t.comp.input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlU: // Override
			return nil
		case tcell.KeyESC: // Clear text and history
			t.comp.input.SetText("", false)
			t.next = 0
			return nil
		case tcell.KeyUp: // Go back in history
			text := t.comp.input.GetText()
			if text != "" && t.next == 0 {
				return event
			}

			if event.Modifiers()&tcell.ModAlt == tcell.ModAlt ||
				event.Modifiers()&tcell.ModShift == tcell.ModShift {
				return event
			}

			l := t.history.Len() - 1
			cmd, ok := t.history.Get(uint(l) - t.next)
			if !ok {
				return event
			}
			t.next += 1

			t.comp.input.SetText("/"+cmd, true)
			return nil
		case tcell.KeyEnter: // Send message or command
			defer func() {
				// Reset history
				t.next = 0
			}()

			// Newline
			if event.Modifiers()&tcell.ModShift == tcell.ModShift ||
				event.Modifiers()&tcell.ModAlt == tcell.ModAlt {
				return event
			}

			text := t.comp.input.GetText()
			if text == "" {
				return nil
			}

			if t.Buffer() == "" {
				t.showError(ErrorNoBuffers)
				return nil
			}

			// Parse as command
			if text[0] == '/' {
				t.parseCommand(text[1:])
				t.comp.input.SetText("", false)
				return nil
			}

			// Prevents message spam
			last := time.Since(t.status.lastMsg)
			if last < time.Duration(msgDelay)*time.Millisecond {
				t.showError(ErrorTypingTooFast)
				t.comp.input.SetText("", false)
				return nil
			}

			// Send the message
			s := t.Active()
			t.sendMessage(Message{
				Sender:    selfSender,
				Buffer:    t.Buffer(),
				Content:   text,
				Timestamp: time.Now(),
				Source:    s.Name(),
			})

			go t.remoteMessage(text)

			t.status.lastMsg = time.Now()
			t.comp.input.SetText("", false)
			return nil
		}
		return event
	})
}

// Sets up global keybinds.
func setupKeybinds(t *TUI) {
	t.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlQ: // Exit program
			t.app.Stop()
		case tcell.KeyCtrlC: // Override to nothing
			return nil
		case tcell.KeyCtrlU: // Show/Hide user list
			toggleUserlist(t)
		case tcell.KeyCtrlB: // Show/Hide buffer list
			toggleBufList(t)
		case tcell.KeyCtrlT: // Changes input between messages and inpit
			if t.status.blockCond() {
				break
			}

			if !t.comp.text.HasFocus() {
				t.app.SetFocus(t.comp.text)
				return nil
			} else {
				t.app.SetFocus(t.comp.input)
				return nil
			}
		case tcell.KeyCtrlS: // Choose a server
			if t.status.blockCond() {
				break
			}

			if !t.comp.servers.HasFocus() {
				t.app.SetFocus(t.comp.servers)
				return nil
			}
		case tcell.KeyCtrlL: // Show help
			if event.Modifiers()&tcell.ModShift == tcell.ModShift ||
				event.Modifiers()&tcell.ModAlt == tcell.ModAlt {
				t.toggleHelp()
			}
		case tcell.KeyCtrlG: // Quick switcher
			if !t.status.blockCond() {
				newQuickSwitchPopup(t)
			}
		case tcell.KeyCtrlK: // Choose a buffer
			if t.status.blockCond() {
				break
			}

			if !t.comp.buffers.HasFocus() {
				t.app.SetFocus(t.comp.buffers)
				return nil
			}
		case tcell.KeyDown: // Go one buffer/server down
			if t.status.blockCond() {
				break
			}

			// Buffer down
			if event.Modifiers()&tcell.ModAlt == tcell.ModAlt {
				curr := t.comp.buffers.GetCurrentItem()
				t.changeBuffer(curr + 1)
			}

			// Server down
			if event.Modifiers()&tcell.ModShift == tcell.ModShift {
				curr := t.comp.servers.GetCurrentItem()
				t.changeServer(curr + 1)
			}
		case tcell.KeyUp: // Go one buffer/server up
			if t.status.blockCond() {
				break
			}

			// Buffer up
			if event.Modifiers()&tcell.ModAlt == tcell.ModAlt {
				curr := t.comp.buffers.GetCurrentItem()
				t.changeBuffer(curr - 1)
			}

			// Server up
			if event.Modifiers()&tcell.ModShift == tcell.ModShift {
				curr := t.comp.servers.GetCurrentItem()
				t.changeServer(curr - 1)
			}
		case tcell.KeyCtrlR: // Reload TUI
			t.app.Sync()
		}
		return event
	})
}

// Creates a new TUI and tview application by its given static data.
// This is needed to run the program in TUI mode.
func New(static cmds.StaticData, debug bool) (*TUI, *tview.Application) {
	areas, comps := setupLayout()
	t := &TUI{
		servers: models.NewTable[string, Server](0),
		comp:    comps,
		area:    areas,
		params:  defaultParams(),
		status: state{
			showingUsers:   false,
			showingBufs:    true,
			showingHelp:    false,
			creatingBuf:    false,
			creatingServer: false,
			deletingServer: false,
			deletingBuffer: false,
			userlist:       models.NewSlice[userlistUser](0),
			serverIndexes:  make([]int, 0),
			lastDate:       time.Now(),
			lastMsg:        time.Now(),
		},
		db:      static.DB,
		history: models.NewSlice[string](0),
	}

	t.params.Verbose = static.Verbose

	// Create the tview application
	app := tview.NewApplication().
		EnableMouse(true).
		SetRoot(t.area.main, true).
		SetFocus(t.area.main)
	t.app = app

	// Render text view
	comps.pages.SwitchToPage(textPage)

	// Render layouts
	renderBuflist(t)
	renderUserlist(t)

	// Setup TUI
	setupKeybinds(t)
	setupHandlers(t)
	setupStyle(t)
	setupInput(t)

	// Local server that runs on the app
	t.servers.Add(localServer, &LocalServer{
		name: localServer,
		bufs: Buffers{
			tabs: models.NewTable[string, *tab](maxBuffers),
		},
	})
	t.focus = localServer
	t.addBuffer(systemBuffer, true)
	l := t.servers.Len()
	t.comp.servers.AddItem(localServer, "System Server", ascii(l), nil)

	// Render help text
	fmt.Fprint(t.comp.help, KeybindHelp[1:])
	fmt.Fprint(t.comp.help, "\n")
	fmt.Fprint(t.comp.help, CommandHelp[1:])
	t.comp.help.ScrollToBeginning()

	// Welcome messages
	info := t.systemMessage()
	info("Welcome to gochat!", cmds.INFO)
	info("Press [green]Ctrl-Alt-L/Ctrl-Shift-L[-] to show help!", cmds.INFO)

	// Debug buffer if necessary
	if debug {
		t.addBuffer(debugBuffer, true)
		info("Packets between client and server will be shown here.", cmds.INFO)
	}

	// Set userlist
	t.comp.users.SetText(defaultUserlist)

	// Change to syste, buffer and restore servers
	t.changeBuffer(int(rootBuffer))
	t.restoreSession()
	t.renderServer(localServer)

	return t, app
}

// Restores all database server entries that are relevant.
func (t *TUI) restoreSession() {
	// Restore servers
	list, err := db.GetAllServers(t.db)
	if err != nil {
		panic(fmt.Sprintf("Failed to restore session! %s", err))
	}

	// Add all database servers
	for _, v := range list {
		err := t.addServer(v.Name, v.Address, v.Port, v.TLS)
		if err != nil {
			panic(err)
		}
		t.addBuffer(defaultBuffer, true)
		welcomeMessage(t)
	}
}
