package tui

import (
	"context"
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

	// Load real data in background to avoid blocking startup
	t.showMessage("Initializing... Loading your Discogs data in background")
	go func() {
		err := t.LoadData()
		if err != nil {
			t.showError(err)
		} else {
			t.DrawPreviewGrid()
		}
	}()

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

// LoadDataWithContext loads the data from all sources with context support
func (t *TUI) LoadDataWithContext(ctx context.Context) error {
	t.Preview.Clear()

	// Add timeout for the entire loading process
	loadCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	t.showMessage("Loading your Discogs data...")

	// Creating collection cards
	collections, err := t.Client.GetCollection()
	if err != nil {
		t.showError(err)
		return err
	}

	t.showMessage(fmt.Sprintf("Loading %d collection items...", len(collections)))

	collectionCards := make([]*tview.Flex, 0, len(collections))
	for i, model := range collections {
		// Check if context is cancelled
		select {
		case <-loadCtx.Done():
			return loadCtx.Err()
		default:
		}

		// Use context-aware method for thumbnail loading
		card, err := t.createReleaseCardPrimitiveWithContext(loadCtx, model)
		if err != nil {
			fmt.Printf("Warning: Failed to create card for %s: %v\n", model.Title, err)
			// Create a text-only card as fallback
			card = t.createTextOnlyCard(model)
		}

		if card != nil {
			card.SetTitle(model.Title)
			card.SetInputCapture(t.openReleaseModal)
			collectionCards = append(collectionCards, card)
		}

		// Update progress
		if i%10 == 0 {
			t.showMessage(fmt.Sprintf("Loaded %d/%d collection items...", i+1, len(collections)))
		}
	}
	t.CollectionPrims = collectionCards

	// Creating wishlist cards
	t.showMessage("Loading wishlist...")
	wants, err := t.Client.GetWishlist()
	if err != nil {
		t.showWarning(fmt.Sprintf("Failed to load wishlist: %v", err))
		// Don't fail completely, just continue without wishlist
		t.WishlistPrims = []*tview.Flex{}
	} else {
		wantCards := make([]*tview.Flex, 0, len(wants))
		for _, model := range wants {
			select {
			case <-loadCtx.Done():
				return loadCtx.Err()
			default:
			}

			card, err := t.createReleaseCardPrimitiveWithContext(loadCtx, model)
			if err != nil {
				fmt.Printf("Warning: Failed to create wishlist card for %s: %v\n", model.Title, err)
				card = t.createTextOnlyCard(model)
			}

			if card != nil {
				card.SetTitle(model.Title)
				wantCards = append(wantCards, card)
			}
		}
		t.WishlistPrims = wantCards
	}

	// Creating order cards
	t.showMessage("Loading orders...")
	orders, err := t.Client.GetOrders()
	if err != nil {
		t.showWarning(fmt.Sprintf("Failed to load orders: %v", err))
		// Don't fail completely, just continue without orders
		t.OrderPrims = []*tview.Flex{}
	} else {
		orderCards := make([]*tview.Flex, 0, len(orders))
		for _, model := range orders {
			select {
			case <-loadCtx.Done():
				return loadCtx.Err()
			default:
			}

			card, err := t.createReleaseCardPrimitiveWithContext(loadCtx, model)
			if err != nil {
				fmt.Printf("Warning: Failed to create order card for %s: %v\n", model.Title, err)
				card = t.createTextOnlyCard(model)
			}

			if card != nil {
				card.SetTitle(model.Title)
				orderCards = append(orderCards, card)
			}
		}
		t.OrderPrims = orderCards
	}

	t.showMessage("✓ Data loading complete!")
	time.AfterFunc(2*time.Second, t.resetMessage)
	return nil
}

// LoadData maintains backward compatibility but with timeout
func (t *TUI) LoadData() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	return t.LoadDataWithContext(ctx)
}

// createReleaseCardPrimitiveWithContext creates a release card with context support
func (t *TUI) createReleaseCardPrimitiveWithContext(ctx context.Context, model dto.ReleaseModel) (*tview.Flex, error) {
	tmpFlex := tview.NewFlex()

	// Use context-aware thumbnail loading with shorter timeout
	thumbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	thumbImg, err := t.Client.GetThumbImageWithContext(thumbCtx, model.ThumbUrl)
	if err != nil {
		// Instead of failing, create a text-only card
		return t.createTextOnlyCard(model), nil
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
	tmpFlex.AddItem(tview.NewTextView().SetText(txt), 0, 2, false)
	tmpFlex.SetBorder(true).SetTitle("Release").SetTitleAlign(tview.AlignLeft)
	return tmpFlex, nil
}

// createTextOnlyCard creates a card without thumbnail as fallback
func (t *TUI) createTextOnlyCard(model dto.ReleaseModel) *tview.Flex {
	tmpFlex := tview.NewFlex()

	txt := fmt.Sprintf(
		`
	%s
	%s | %d | %s
	%s

	Condition: %s
	Sleeve Condition: %s
	Genre: %s
	Style: %s
	
	[Thumbnail unavailable]
	`,
		model.Title,
		model.Artist, model.Year, model.Label,
		model.Format,
		model.MediaCondition,
		model.SleeveCondition,
		model.Genre,
		model.Style,
	)

	tmpFlex.AddItem(tview.NewTextView().SetText(txt), 0, 1, false)
	tmpFlex.SetBorder(true).SetTitle("Release").SetTitleAlign(tview.AlignLeft)
	return tmpFlex
}

// StartWithContext starts the TUI with context support
func (t *TUI) StartWithContext(ctx context.Context) error {
	// Set up autoupdate with context
	updateFreq := time.Duration(t.Config.UpdateFreq) * time.Second
	ticker := time.NewTicker(updateFreq)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return // Exit goroutine when context is cancelled
			case <-ticker.C:
				if time.Since(t.LastUpdated) >= updateFreq {
					// Update with context and timeout
					updateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					if err := t.LoadDataWithContext(updateCtx); err != nil {
						t.showError(err)
					} else {
						t.DrawPreviewGrid()
					}
					cancel()
				}
			}
		}
	}()

	return t.App.SetRoot(t.Pages, true).EnableMouse(true).Run()
}
