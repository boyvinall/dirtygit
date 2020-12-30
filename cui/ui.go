package cui

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"sync/atomic"

	"github.com/jroimartin/gocui"

	"github.com/boyvinall/dirtygit/scanner"
)

const (
	vRepo     = "repo"
	vDiff     = "diff"
	vScanning = "scanning"
)

type ui struct {
	scan         chan struct{}
	config       *scanner.Config
	repositories scanner.MultiGitStatus
	currentRepo  string

	scanning uint32
}

func Run(config *scanner.Config) error {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.Cursor = true
	g.Mouse = true

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
	if maxY < 10 {
		return errors.New("Need bigger screen")
	}
	numRepositories := len(u.repositories)
	divide := min(numRepositories+2, maxY/2)

	repo, err := g.SetView(vRepo, 0, 0, maxX-1, divide)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	repo.Title = "Repositories"
	repo.Highlight = true
	repo.SelBgColor = gocui.ColorGreen
	repo.SelFgColor = gocui.ColorBlack
	if g.CurrentView() == nil {
		g.SetCurrentView(repo.Name())
	}
	diff, err := g.SetView(vDiff, 0, divide+1, maxX-1, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	diff.Title = "Diff"

	if atomic.LoadUint32(&u.scanning) > 0 {
		scanning, err := g.SetView(vScanning, maxX/2-10, maxY/2-1, maxX/2+10, maxY/2+1)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		if err == gocui.ErrUnknownView {
			fmt.Fprint(scanning, "  Scanning...")
		}
	} else {
		g.DeleteView(vScanning)
	}

	return nil
}

func (u *ui) initKeybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, u.quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 'q', gocui.ModNone, u.quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, u.nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyArrowUp, gocui.ModNone, u.up); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyArrowDown, gocui.ModNone, u.down); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 's', gocui.ModNone, u.requestScan); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 'e', gocui.ModNone, u.edit); err != nil {
		return err
	}
	return nil
}

func (u *ui) nextView(g *gocui.Gui, v *gocui.View) error {
	if v == nil {
		g.SetCurrentView(vRepo)
		return nil
	}
	switch v.Name() {
	case vRepo:
		g.SetCurrentView(vDiff)
	default:
		g.SetCurrentView(vRepo)
	}
	return nil
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
		if len(st) > 0 {
			s += fmt.Sprintln(" SW")
			s += fmt.Sprintln("-----")
			paths := make([]string, 0, len(st))
			for path := range st {
				paths = append(paths, path)
			}
			sort.Strings(paths)
			for _, path := range paths {
				status := st[path]
				s += fmt.Sprintf(" %c%c  %s\n", status.Staging, status.Worktree, path)
			}
		} else {

		}
	}
	diff, _ := g.View(vDiff)
	diff.Clear()
	fmt.Fprint(diff, s)
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
	u.requestScan(g, nil)

	update := func() {
		g.Update(func(g *gocui.Gui) error {
			return nil
		})
	}
	for {
		select {
		case <-u.scan:
			atomic.StoreUint32(&u.scanning, 1)
			update()

			var err error
			u.repositories, err = scanner.Scan(u.config)
			if err == nil {
				u.updateDirList(g)
			}

			atomic.StoreUint32(&u.scanning, 0)
			u.flushScan()
			update()
		}
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
