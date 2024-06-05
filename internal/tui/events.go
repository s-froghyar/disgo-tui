package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/s-froghyar/disgo-tui/internal/client"
)

func (t *TUI) sourceSelected(_ int, _ string, _ string, shortcut rune) {
	switch shortcut {
	case '0':
		t.SelectedSource = client.CollectionSource
	case '1':
		t.SelectedSource = client.WishlistSource
	case '2':
		t.SelectedSource = client.OrdersSource
	case 'q':
		return
	}

	t.PreviewPosition = [2]int{0, 0}
	t.DrawPreviewGrid()
}

func (t *TUI) focusOnPreview(src client.DataSource) func() {
	return func() {
		t.queueUpdateDraw(func() {
			t.App.SetFocus(t.Preview)
			switch src {
			case client.CollectionSource:
				if len(t.CollectionPrims) > 0 {
					t.App.SetFocus(t.CollectionPrims[0])
				}
			case client.WishlistSource:
				if len(t.WishlistPrims) > 0 {
					t.App.SetFocus(t.WishlistPrims[0])
				}
			case client.OrdersSource:
				if len(t.OrderPrims) > 0 {
					t.App.SetFocus(t.OrderPrims[0])
				}
			}
		})
	}
}

func (t *TUI) setUpInputCaptures() {
	t.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		// app navigation
		case KeyMapping[KeyMenuOp]:
			t.App.SetFocus(t.Navigation)
		case KeyMapping[KeyPreviewOp]:
			t.App.SetFocus(t.Preview)
			primIndex := t.PreviewPosition[0] + t.PreviewPosition[1]
			switch t.SelectedSource {
			case client.CollectionSource:
				if len(t.CollectionPrims) > 0 {
					t.App.SetFocus(t.CollectionPrims[primIndex])
				}
			case client.WishlistSource:
				if len(t.WishlistPrims) > 0 {
					t.App.SetFocus(t.WishlistPrims[primIndex])
				}
			case client.OrdersSource:
				if len(t.OrderPrims) > 0 {
					t.App.SetFocus(t.OrderPrims[primIndex])
				}
			}

		// preview navigation
		case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight:
			t.handlePreviewNavigation(event.Key())
		}
		return event
	})
}

func (t *TUI) handlePreviewNavigation(k tcell.Key) {
	if !t.Preview.HasFocus() {
		return
	}
	potentialPosition := t.PreviewPosition
	switch k {
	case tcell.KeyUp:
		if potentialPosition[0] > 0 {
			potentialPosition[0] -= t.Config.Grid.NumOfRows
		}
	case tcell.KeyDown:
		if potentialPosition[0] < t.Config.Grid.NumOfRows-1 {
			potentialPosition[0] += t.Config.Grid.NumOfRows
		}
	case tcell.KeyLeft:
		if potentialPosition[1] > 0 {
			potentialPosition[1]--
		}
	case tcell.KeyRight:
		if potentialPosition[1] < t.Config.Grid.NumOfCols-1 {
			potentialPosition[1]++
		}
	}

	primIndex := potentialPosition[0] + potentialPosition[1]
	overstep := false
	switch t.SelectedSource {
	case client.CollectionSource:
		if len(t.CollectionPrims) > 0 {
			if primIndex < len(t.CollectionPrims) {
				t.App.SetFocus(t.CollectionPrims[primIndex])
			} else {
				overstep = true
			}
		}
	case client.WishlistSource:
		if len(t.WishlistPrims) > 0 {
			if primIndex < len(t.WishlistPrims) {
				t.App.SetFocus(t.WishlistPrims[primIndex])
			} else {
				overstep = true
			}
		}
	case client.OrdersSource:
		if len(t.OrderPrims) > 0 {
			if primIndex < len(t.OrderPrims) {
				t.App.SetFocus(t.OrderPrims[primIndex])
			} else {
				overstep = true
			}
		}
	}
	if !overstep {
		t.PreviewPosition = potentialPosition
	}
}

func (t *TUI) openReleaseModal(key *tcell.EventKey) *tcell.EventKey {
	switch key.Key() {
	case tcell.KeyEnter:
		infobox := tview.NewModal().
			AddButtons([]string{"Close"}).
			SetDoneFunc(func(_ int, _ string) {
				t.Pages.SwitchToPage("main")
				t.App.SetFocus(t.Preview)
				t.handlePreviewNavigation(tcell.KeyEnd)
			}).
			SetText("Lorem Ipsum Is A Pain")

		t.Pages.AddAndSwitchToPage("modal", infobox, true)

	}
	return key
}
