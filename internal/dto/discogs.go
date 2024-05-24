package dto

import (
	"fmt"
	"strings"
)

type DiscogsPaginationDto struct {
	Page  int               `json:"page"`
	Pages int               `json:"pages"`
	Per   int               `json:"per_page"`
	Items int               `json:"items"`
	Urls  map[string]string `json:"urls"`
}

type NoteDto struct {
	FieldId int    `json:"field_id"`
	Value   string `json:"value"`
}

type DiscogsReleaseFormatDto struct {
	Name         string   `json:"name"`
	Qty          string   `json:"qty"`
	Text         string   `json:"text"`
	Descriptions []string `json:"descriptions"`
}

type DiscogsReleaseArtistDto struct {
	Name        string `json:"name"`
	Anv         string `json:"anv"`
	Join        string `json:"join"`
	Role        string `json:"role"`
	ResourceUrl string `json:"resource_url"`
	Id          int    `json:"id"`
	Tracks      string `json:"tracks"`
}

type DiscogsReleaseLabelDto struct {
	Name           string `json:"name"`
	CatNo          string `json:"catno"`
	ResourceUrl    string `json:"resource_url"`
	Id             int    `json:"id"`
	EntityType     string `json:"entity_type"`
	EntityTypeName string `json:"entity_type_name"`
}

type DiscogsBasicInformationDto struct {
	Id          int                       `json:"id"`
	MasterId    int                       `json:"master_id"`
	MasterUrl   string                    `json:"master_url"`
	ResourceUrl string                    `json:"resource_url"`
	Thumb       string                    `json:"thumb"`
	CoverImage  string                    `json:"cover_image"`
	Title       string                    `json:"title"`
	Year        int                       `json:"year"`
	Formats     []DiscogsReleaseFormatDto `json:"formats"`
	Artists     []DiscogsReleaseArtistDto `json:"artists"`
	Labels      []DiscogsReleaseLabelDto  `json:"labels"`
	Genres      []string                  `json:"genres"`
	Styles      []string                  `json:"styles"`
}

type DiscogsReleaseDto[T any] struct {
	Id               int                        `json:"id"`
	InstanceID       int                        `json:"instance_id"`
	Rating           uint8                      `json:"rating"`
	DateAdded        string                     `json:"date_added"`
	FolderId         int                        `json:"folder_id"`
	Notes            T                          `json:"notes"`
	BasicInformation DiscogsBasicInformationDto `json:"basic_information"`
}

type PaginationBaseDto struct {
	Pagination DiscogsPaginationDto `json:"pagination"`
}

type CollectionBaseDto struct {
	PaginationBaseDto
	Releases []DiscogsReleaseDto[[]NoteDto] `json:"releases"`
}
type WishlistBaseDto struct {
	PaginationBaseDto
	Wants []DiscogsReleaseDto[string] `json:"wants"`
}
type OrdersBaseDto struct {
	PaginationBaseDto
	Orders []DiscogsReleaseDto[string] `json:"orders"`
}

type ReleaseModel struct {
	Title           string
	Rating          uint8
	Year            int
	Artist          string
	Label           string
	Genre           string
	Style           string
	MediaCondition  string
	SleeveCondition string
	Note            string
	ThumbUrl        string
	Format          string
}

func MapCollectionReleases(releases []DiscogsReleaseDto[[]NoteDto]) ([]ReleaseModel, error) {
	data := make([]ReleaseModel, len(releases))
	for i, release := range releases {
		tmp := ReleaseModel{
			Title:    release.BasicInformation.Title,
			Rating:   release.Rating,
			Year:     release.BasicInformation.Year,
			Artist:   release.BasicInformation.Artists[0].Name,
			Label:    release.BasicInformation.Labels[0].Name,
			Genre:    strings.Join(release.BasicInformation.Genres, ", "),
			Style:    strings.Join(release.BasicInformation.Styles, ", "),
			ThumbUrl: release.BasicInformation.Thumb,
		}
		// Map notes to conditions
		for _, note := range release.Notes {
			switch note.FieldId {
			case 1:
				tmp.MediaCondition = note.Value
			case 2:
				tmp.SleeveCondition = note.Value
			case 3:
				tmp.Note = note.Value
			}
		}
		// Map formats to a single string
		formats := make([]string, len(release.BasicInformation.Formats))
		for j, format := range release.BasicInformation.Formats {
			formats[j] = fmt.Sprintf("%sx %s: %s", format.Qty, format.Name, strings.Join(format.Descriptions, "-"))
		}
		tmp.Format = strings.Join(formats, "\n\t")

		data[i] = tmp
	}
	return data, nil
}

func MapWishlistReleases(releases []DiscogsReleaseDto[string]) ([]ReleaseModel, error) {
	data := make([]ReleaseModel, len(releases))
	for i, release := range releases {
		tmp := ReleaseModel{
			Title:    release.BasicInformation.Title,
			Rating:   release.Rating,
			Year:     release.BasicInformation.Year,
			Artist:   release.BasicInformation.Artists[0].Name,
			Label:    release.BasicInformation.Labels[0].Name,
			Genre:    strings.Join(release.BasicInformation.Genres, ", "),
			Style:    strings.Join(release.BasicInformation.Styles, ", "),
			ThumbUrl: release.BasicInformation.Thumb,
			Note:     release.Notes,
		}
		// Map formats to a single string
		formats := make([]string, len(release.BasicInformation.Formats))
		for j, format := range release.BasicInformation.Formats {
			formats[j] = fmt.Sprintf("%sx %s: %s", format.Qty, format.Name, strings.Join(format.Descriptions, "-"))
		}
		tmp.Format = strings.Join(formats, "\n\t")

		data[i] = tmp
	}
	return data, nil
}
