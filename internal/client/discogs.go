package client

import (
	"encoding/json"
	"fmt"

	"github.com/s-froghyar/disgo-tui/internal/dto"
)

type DataSource int

const (
	CollectionSource DataSource = iota
	WishlistSource
	OrdersSource

	// CollectionURL is the URL for the user's collection.
	CollectionURL string = "https://api.discogs.com/users/%s/collection/folders/0/releases"
	// WishlistURL is the URL for the user's wishlist.
	WishlistURL string = "https://api.discogs.com/users/%s/wants"
	// OrdersURL is the URL for the user's orders.
	OrdersURL string = "https://api.discogs.com/users/%s/orders"
)

func (c *DiscogsClient) GetCollection() ([]dto.ReleaseModel, error) {
	// Get the collection
	resp, err := c.Get(fmt.Sprintf(CollectionURL, c.Identity.Username))
	if err != nil {
		fmt.Printf("Error at Get: %v \n", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the response
	var collectionDto dto.CollectionBaseDto
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&collectionDto)
	if err != nil {
		fmt.Printf("Error at decoding body: %v \n", err)
		return nil, err
	}

	// Map the DTO to the model
	collection, err := dto.MapCollectionReleases(collectionDto.Releases)
	if err != nil {
		fmt.Printf("Error at MapReleases: %v \n", err)
		return nil, err
	}
	return collection, nil
}

func (c *DiscogsClient) GetWishlist() ([]dto.ReleaseModel, error) {
	// Get the wish list
	resp, err := c.Get(fmt.Sprintf(WishlistURL, c.Identity.Username))
	if err != nil {
		fmt.Printf("Error at Get: %v \n", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the response
	var wantsDto dto.WishlistBaseDto
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&wantsDto)
	if err != nil {
		fmt.Printf("Error at decoding body: %v \n", err)
		return nil, err
	}

	// Map the DTO to the model
	wants, err := dto.MapWishlistReleases(wantsDto.Wants)
	if err != nil {
		fmt.Printf("Error at MapReleases: %v \n", err)
		return nil, err
	}
	return wants, nil
}

func (c *DiscogsClient) GetOrders() ([]dto.ReleaseModel, error) {
	// Get the orders
	resp, err := c.Get(fmt.Sprintf(OrdersURL, c.Identity.Username))
	if err != nil {
		fmt.Printf("Error at Get: %v \n", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the response
	var wantsDto dto.WishlistBaseDto
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&wantsDto)
	if err != nil {
		fmt.Printf("Error at decoding body: %v \n", err)
		return nil, err
	}

	// Map the DTO to the model
	wants, err := dto.MapWishlistReleases(wantsDto.Wants)
	if err != nil {
		fmt.Printf("Error at MapReleases: %v \n", err)
		return nil, err
	}
	return wants, nil
}
