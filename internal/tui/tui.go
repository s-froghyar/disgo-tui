package tui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/s-froghyar/disgo-tui/configs"
	"github.com/s-froghyar/disgo-tui/internal/client"
	"github.com/s-froghyar/disgo-tui/internal/dto"
)

type (
	// KeyOp defines digo-tui specific hotkey operations.
	KeyOp int16
)

var (
	MenuTitle    = fmt.Sprintf("Menu [ %s ]", tcell.KeyNames[KeyMapping[KeyMenuOp]])
	PreviewTitle = fmt.Sprintf("Preview [ %s ]", tcell.KeyNames[KeyMapping[KeyPreviewOp]])

	// TitleFooterView is the title for Footer view.
	FooterText = "Navigate: Arrow keys [Up, Down, Right, Left] · Preview specific: Return [ Enter ] · Exit [ Ctrl-C ]"
)

type TUI struct {
	Client *client.DiscogsClient
	Config *configs.AppConfig

	App        *tview.Application
	Pages      *tview.Pages
	Grid       *tview.Grid
	Navigation *tview.List
	Footer     *tview.TextView

	Preview         *tview.Grid
	CollectionPrims []*tview.Flex
	WishlistPrims   []*tview.Flex
	OrderPrims      []*tview.Flex

	SelectedSource  client.DataSource
	PreviewPosition [2]int
	LastUpdated     time.Time
}

// New creates a new TUI instance.
func New(c *client.DiscogsClient, config *configs.AppConfig) *TUI {
	t := TUI{}
	t.App = tview.NewApplication()
	t.Client = c
	t.Config = config

	// menu list
	t.Navigation = tview.NewList()
	t.Navigation.SetBorder(true).SetTitle(MenuTitle).SetBackgroundColor(tcell.ColorBlack)
	t.Navigation.
		AddItem("Collection", "Display the releases in your Collection", '0', t.focusOnPreview(client.CollectionSource)).
		AddItem("Wish list", "Display the releases in your Wish list", '1', t.focusOnPreview(client.WishlistSource)).
		AddItem("Orders", "Check the status of your Orders", '2', t.focusOnPreview(client.OrdersSource)).
		AddItem("Quit", "Press to exit", 'q', func() { t.App.Stop() })
	t.Navigation.SetChangedFunc(t.sourceSelected)
	leftPanel := tview.NewGrid().
		SetRows(0, 0).
		SetBorders(false).
		AddItem(t.Navigation, 0, 0, 1, 1, 0, 0, true).
		AddItem(getLogoPrimitive(), 1, 0, 1, 1, 0, 0, false)

	// preview grid
	rowSlice := make([]int, config.Grid.NumOfRows)
	colSlice := make([]int, config.Grid.NumOfCols)

	t.Preview = tview.NewGrid().SetRows(rowSlice...).SetColumns(colSlice...)
	t.Preview.SetTitle(PreviewTitle)
	t.Preview.SetBorder(true)

	t.Footer = tview.NewTextView().SetTextAlign(tview.AlignCenter).SetText(FooterText).SetTextColor(tcell.ColorGray)

	t.Grid = tview.NewGrid().
		SetRows(0, 2).
		SetColumns(40, 0).
		SetBorders(false).
		AddItem(leftPanel, 0, 0, 1, 1, 0, 0, true).
		AddItem(t.Preview, 0, 1, 1, 1, 0, 0, false).
		AddItem(t.Footer, 1, 0, 1, 2, 0, 0, false)

	t.Pages = tview.NewPages().AddPage("main", t.Grid, true, true)

	t.setUpInputCaptures()

	// Load real data
	err := t.LoadData()
	if err != nil {
		panic(err)
	}

	t.DrawPreviewGrid()

	return &t
}

// Start starts terminal user interface application.
func (t *TUI) Start() error {
	// Set up autoupdate
	updateFreq := time.Duration(t.Config.UpdateFreq) * time.Second
	ticker := time.NewTicker(updateFreq)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				if time.Since(t.LastUpdated) >= updateFreq {
					// update all data
					t.showMessage(fmt.Sprintf("ticker triggered and we have this selected: %v", t.SelectedSource))
					t.DrawPreviewGrid()
				}
			}
		}
	}()

	return t.App.SetRoot(t.Pages, true).EnableMouse(true).Run()
}

// Stop stops terminal user interface application.
func (t *TUI) Stop() {
	t.App.Stop()
}

// nolint
func (tui *TUI) queueUpdate(f func()) {
	go func() {
		tui.App.QueueUpdate(f)
	}()
}

func (tui *TUI) queueUpdateDraw(f func()) {
	go func() {
		tui.App.QueueUpdateDraw(f)
	}()
}

func (t *TUI) resetMessage() {
	t.queueUpdateDraw(func() {
		t.Footer.SetText(FooterText).SetTextColor(tcell.ColorGray)
	})
}

func (t *TUI) showMessage(msg string) {
	t.queueUpdateDraw(func() {
		t.Footer.SetText(msg).SetTextColor(tcell.ColorGreen)
	})
	go time.AfterFunc(3*time.Second, t.resetMessage)
}

func (t *TUI) showWarning(msg string) {
	t.queueUpdateDraw(func() {
		t.Footer.SetText(msg).SetTextColor(tcell.ColorYellow)
	})
	go time.AfterFunc(3*time.Second, t.resetMessage)
}

func (t *TUI) showError(err error) {
	t.queueUpdateDraw(func() {
		t.Preview.Clear()
		t.Footer.SetText(err.Error()).SetTextColor(tcell.ColorRed)
	})
	go time.AfterFunc(50*time.Second, t.resetMessage)
}

// LoadData loads the data from all the sources and creates the primitives to be displayed
func (t *TUI) LoadData() error {
	t.Preview.Clear()

	// Creating collection cards
	collections, err := t.Client.GetCollection()
	if err != nil {
		t.showError(err)
		return err
	}
	collectionCards := make([]*tview.Flex, len(collections))
	for i, model := range collections {
		if collectionCards[i], err = t.createReleaseCardPrimitive(model); err != nil {
			fmt.Printf("Error at createReleaseCardPrimitive: %v \n", err)
			t.showError(err)
		}
		collectionCards[i].SetTitle(model.Title)
		collectionCards[i].SetInputCapture(t.openReleaseModal)

	}
	t.CollectionPrims = collectionCards

	// Creating wishlist cards
	wants, err := t.Client.GetWishlist()
	if err != nil {
		t.showError(err)
		return err
	}
	wantCards := make([]*tview.Flex, len(wants))
	for i, model := range wants {
		if wantCards[i], err = t.createReleaseCardPrimitive(model); err != nil {
			fmt.Printf("Error at createReleaseCardPrimitive: %v \n", err)
			t.showError(err)
		}
		wantCards[i].SetTitle(model.Title)
	}
	t.WishlistPrims = wantCards

	// Creating order cards
	orders, err := t.Client.GetOrders()
	if err != nil {
		t.showError(err)
		return err
	}
	orderCards := make([]*tview.Flex, len(orders))
	for i, model := range orders {
		if orderCards[i], err = t.createReleaseCardPrimitive(model); err != nil {
			fmt.Printf("Error at createReleaseCardPrimitive: %v \n", err)
			t.showError(err)
			return err
		}
		orderCards[i].SetTitle(model.Title)
	}
	t.OrderPrims = orderCards

	return nil
}

func (t *TUI) createReleaseCardPrimitive(model dto.ReleaseModel) (*tview.Flex, error) {
	tmpFlex := tview.NewFlex() //.SetDirection(tview.FlexRow)
	thumbImg, err := t.Client.GetThumbImage(model.ThumbUrl)
	if err != nil {
		fmt.Printf("Error at GetThumbImage: %v \n", err)
		return nil, err
	}

	// Card content
	txt := fmt.Sprintf(
		`
	%s
	%s | %d | %s
	%s


	Condition: %s
	Sleeve Condition: %s
	Genre: %s
	Style: %s
	`,
		model.Title,
		model.Artist, model.Year, model.Label,
		model.Format,
		model.MediaCondition,
		model.SleeveCondition,
		model.Genre,
		model.Style,
	)

	tmpFlex.AddItem(tview.NewImage().SetImage(thumbImg), 0, 1, false)
	tmpFlex.AddItem(tview.NewTextView().SetText(
		txt,
	), 0, 2, false)
	tmpFlex.SetBorder(true).SetTitle("Release").SetTitleAlign(tview.AlignLeft)
	return tmpFlex, nil
}

func (t *TUI) DrawPreviewGrid() {
	t.queueUpdateDraw(func() {
		t.Preview.Clear()

		var cards []*tview.Flex
		switch t.SelectedSource {
		case client.CollectionSource:
			cards = t.CollectionPrims
		case client.WishlistSource:
			cards = t.WishlistPrims
		case client.OrdersSource:
			cards = t.OrderPrims
		}
		for i := range len(cards) {
			row := i / t.Config.Grid.NumOfCols
			column := i % t.Config.Grid.NumOfCols

			t.Preview.AddItem(cards[i], row, column, 1, 1, 0, 0, false)
		}
		t.LastUpdated = time.Now()
	})
}
