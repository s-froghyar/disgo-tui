# Disgo TUI
Terminal UI for the Discogs collections.

Current version can only display user's collection, want list and orders.

## Test it out

Run `go build ./cmd/main.go` to generate the executable then run it.

On first run you should be prompted to sign in via Discogs to grant permission to your data. Once that's done, the TUI app will open with your collections on display. You also need to save the displayed tokens in your `.zshrc` or `.bashrc` file.

## Future Requirements

As of writing, the navigation should work, but not much interaction is implemented. I will list here the potential features to be added in the future:

- <b>Relesea Detail modal</b> - A modal to open for the release chosen to display more details about it as well as additional options to interact (eg. remove from collection / add to specific list)
- <b>Search</b> - Search tab to allow the user to browse the discogs catalogue from the TUI and add them to specific lists or the collection.
- <b>Order management</b> - Allow messaging for the orders and a more fine grained management options

