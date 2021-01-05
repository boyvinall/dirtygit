package ui

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"sync/atomic"
	"time"

	"github.com/jroimartin/gocui"

	"github.com/boyvinall/dirtygit/scanner"
)

const (
	vRepo     = "repo"
	vStatus   = "status"
	vScanning = "scanning"
	vError    = "error"
	vLog      = "log"
)

type ui struct {
	scan         chan struct{}
	config       *scanner.Config
	repositories scanner.MultiGitStatus

	scanning     uint32
	scanProgress int
	err          error
}

func Run(config *scanner.Config) error {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		return err
	}
	defer g.Close()

	g.Cursor = true
	// g.Mouse = true

	u := &ui{}
	u.config = config
	u.scan = make(chan struct{}, 1)
	go u.Run(g)

	g.SetManager(u)

	if err := u.initKeybindings(g); err != nil {
		return err
	}
	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		return err
	}

	return nil
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func (u *ui) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if maxY < 20 {
		return errors.New("Need bigger screen")
	}
	numRepositories := len(u.repositories)
	divide := min(numRepositories+2, maxY/2)

	repo, err := g.SetView(vRepo, 0, 0, maxX-1, divide)
	if err == gocui.ErrUnknownView {
		repo.Title = " Repositories "
		repo.Highlight = true
		repo.SelBgColor = gocui.ColorGreen
		repo.SelFgColor = gocui.ColorBlack
	} else if err != nil {
		return err
	}
	if g.CurrentView() == nil {
		_, err = g.SetCurrentView(vRepo)
		if err != nil {
			return err
		}
	}

	status, err := g.SetView(vStatus, 0, divide+1, maxX-1, maxY-11)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	status.Title = " Status "

	logwindow, err := g.SetView(vLog, 0, maxY-10, maxX, maxY-1)
	if err == gocui.ErrUnknownView {
		logwindow.Title = " Log "
		logwindow.Autoscroll = true
		logwindow.Wrap = true
		log.SetOutput(logwindow)
	} else if err != nil {
		// if err != nil && err != gocui.ErrUnknownView {
		return err
	} else {
		// _, y := logwindow.Size()
		// logwindow.SetCursor(0, y-1)
	}

	if atomic.LoadUint32(&u.scanning) > 0 {
		var scanning *gocui.View
		scanning, err = g.SetView(vScanning, maxX/2-10, maxY/2-1, maxX/2+10, maxY/2+1)
		scanning.Title = " Scanning "
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		if err == gocui.ErrUnknownView {
			u.scanProgress = 1
			_, err = g.SetCurrentView(vScanning)
			return err
		}
		u.updateScanProgress(g, scanning)
	} else {
		err = g.DeleteView(vScanning)
		if err == nil {
			_, err = g.SetCurrentView(vRepo)
			if err != nil {
				return err
			}
		}
		if err != gocui.ErrUnknownView {
			return err
		}
	}
	if u.err != nil {
		var errorView *gocui.View
		errorView, err = g.SetView(vError, maxX/8, maxY/2-2, 7*maxX/8, maxY/2+2)
		errorView.Title = " Error "
		errorView.Wrap = true
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		if err == gocui.ErrUnknownView {
			fmt.Fprint(errorView, u.err)
			_, err = g.SetCurrentView(vError)
			return err
		}
	} else {
		err = g.DeleteView(vError)
		if err == nil {
			_, err = g.SetCurrentView(vRepo)
			if err != nil {
				return err
			}
		}
		if err != gocui.ErrUnknownView {
			return err
		}
	}

	return nil
}

func (u *ui) updateScanProgress(g *gocui.Gui, v *gocui.View) {
	x, y := v.Cursor()
	err := v.SetCursor(x+u.scanProgress, y)
	if err != nil {
		u.scanProgress = -u.scanProgress
	}
}

func (u *ui) initKeybindings(g *gocui.Gui) error {
	type keybinding struct {
		viewname string
		key      interface{}
		mod      gocui.Modifier
		handler  func(*gocui.Gui, *gocui.View) error
	}
	for _, k := range []keybinding{
		{"", gocui.KeyCtrlC, gocui.ModNone, u.quit},
		{"", 'q', gocui.ModNone, u.quit},
		{vStatus, gocui.KeyTab, gocui.ModNone, u.nextView},
		{vRepo, gocui.KeyTab, gocui.ModNone, u.nextView},
		{vLog, gocui.KeyTab, gocui.ModNone, u.nextView},
		{"", gocui.KeyArrowUp, gocui.ModNone, u.up},
		{"", gocui.KeyArrowDown, gocui.ModNone, u.down},
		{"", 's', gocui.ModNone, u.requestScan},
		{"", 'e', gocui.ModNone, u.edit},
	} {
		if err := g.SetKeybinding(k.viewname, k.key, k.mod, k.handler); err != nil {
			return err
		}
	}
	return nil
}

func (u *ui) nextView(g *gocui.Gui, v *gocui.View) error {
	if v == nil {
		_, err := g.SetCurrentView(vRepo)
		return err
	}
	switch v.Name() {
	case vRepo:
		_, err := g.SetCurrentView(vStatus)
		return err
	case vStatus:
		_, err := g.SetCurrentView(vLog)
		return err
	default:
		_, err := g.SetCurrentView(vRepo)
		return err
	}
}

func (u *ui) quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (u *ui) up(g *gocui.Gui, v *gocui.View) error {
	v.MoveCursor(0, -1, false)
	if v.Name() == vRepo {
		u.updateDiff(g)
	}
	return nil
}

func (u *ui) down(g *gocui.Gui, v *gocui.View) error {
	v.MoveCursor(0, 1, false)
	if v.Name() == vRepo {
		u.updateDiff(g)
	}
	return nil
}

func (u *ui) getCurrentRepo(g *gocui.Gui) string {
	repo, err := g.View(vRepo)
	if err != nil {
		return ""
	}

	_, y := repo.Cursor()
	currentRepo, err := repo.Line(y)
	if err != nil {
		return ""
	}

	return currentRepo
}

func (u *ui) edit(g *gocui.Gui, v *gocui.View) error {
	currentRepo := u.getCurrentRepo(g)
	if currentRepo == "" {
		return nil
	}
	cmd := exec.Command("code", currentRepo)
	err := cmd.Run()
	return err
}

func (u *ui) updateDiff(g *gocui.Gui) {
	currentRepo := u.getCurrentRepo(g)
	st, ok := u.repositories[currentRepo]
	s := ""
	if ok {
		if len(st.Status) > 0 {
			s += fmt.Sprintln(" SW")
			s += fmt.Sprintln("-----")
			paths := make([]string, 0, len(st.Status))
			for path := range st.Status {
				paths = append(paths, path)
			}
			sort.Strings(paths)
			for _, path := range paths {
				status := st.Status[path]
				s += fmt.Sprintf(" %c%c  %s\n", status.Staging, status.Worktree, path)
			}
		} else {

		}
	}
	status, _ := g.View(vStatus)
	status.Clear()
	fmt.Fprint(status, s)
}

func (u *ui) requestScan(g *gocui.Gui, v *gocui.View) error {
	u.flushScan()
	u.scan <- struct{}{}
	return nil
}

func (u *ui) updateDirList(g *gocui.Gui) {
	repo, err := g.View(vRepo)
	if err != nil {
		return
	}
	repo.Clear()
	paths := make([]string, 0, len(u.repositories))
	for r := range u.repositories {
		paths = append(paths, r)
	}
	sort.Strings(paths)
	for i := range paths {
		if i > 0 {
			fmt.Fprintln(repo, "")
		}
		fmt.Fprint(repo, paths[i])
	}
	u.updateDiff(g)
}

func (u *ui) Run(g *gocui.Gui) {
	err := u.requestScan(g, nil)
	if err != nil {
		return
	}

	update := func() {
		g.Update(func(g *gocui.Gui) error {
			return nil
		})
	}

	for range u.scan {
		atomic.StoreUint32(&u.scanning, 1)
		update()

		t := time.NewTicker(100 * time.Millisecond)
		scanDone := make(chan struct{})
		go func() {
			u.repositories, u.err = scanner.Scan(u.config)
			if u.err == nil {
				u.updateDirList(g)
			}
			scanDone <- struct{}{}
			update()
		}()
	updateLoop:
		for {
			select {
			case <-t.C:
				update()
			case <-scanDone:
				t.Stop()
				break updateLoop
			}
		}

		atomic.StoreUint32(&u.scanning, 0)
		u.flushScan()
		update()
	}
}

func (u *ui) flushScan() {
	for {
		select {
		case <-u.scan:
		default:
			return
		}
	}
}
