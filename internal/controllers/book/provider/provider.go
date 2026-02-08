package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andre-felipe-wonsik-alves/bookworm-back/internal/models"
)

var ErrNoResults = fmt.Errorf("nenhum resultado encontrado")

type ExternalBookProvider interface {
	SearchByISBN(ctx context.Context, isbn string) (*models.Book, error)
	SearchByQuery(ctx context.Context, query string) ([]models.Book, error)
}

type CompositeProvider struct {
	providers []ExternalBookProvider
}

func NewCompositeProvider(providers ...ExternalBookProvider) *CompositeProvider {
	return &CompositeProvider{providers: providers}
}

func (c *CompositeProvider) SearchByISBN(ctx context.Context, isbn string) (*models.Book, error) {
	var lastErr error
	for _, p := range c.providers {
		book, err := p.SearchByISBN(ctx, isbn)
		if err == nil && book != nil {
			return book, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrNoResults
}

func (c *CompositeProvider) SearchByQuery(ctx context.Context, query string) ([]models.Book, error) {
	var lastErr error
	for _, p := range c.providers {
		books, err := p.SearchByQuery(ctx, query)
		if err == nil && len(books) > 0 {
			return books, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrNoResults
}

type GoogleBooksProvider struct {
	client *http.Client
}

func NewGoogleBooksProvider(timeout time.Duration) *GoogleBooksProvider {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &GoogleBooksProvider{client: &http.Client{Timeout: timeout}}
}

func (p *GoogleBooksProvider) SearchByISBN(ctx context.Context, isbn string) (*models.Book, error) {
	endpoint := "https://www.googleapis.com/books/v1/volumes?q=isbn:" + url.QueryEscape(isbn)
	books, err := p.fetch(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	if len(books) == 0 {
		return nil, ErrNoResults
	}
	book := books[0]
	book.ISBN = isbn
	book.CacheSource = "google_books"
	return &book, nil
}

func (p *GoogleBooksProvider) SearchByQuery(ctx context.Context, query string) ([]models.Book, error) {
	endpoint := "https://www.googleapis.com/books/v1/volumes?q=" + url.QueryEscape(query)
	books, err := p.fetch(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	if len(books) == 0 {
		return nil, ErrNoResults
	}
	for i := range books {
		books[i].CacheSource = "google_books"
	}
	return books, nil
}

func (p *GoogleBooksProvider) fetch(ctx context.Context, endpoint string) ([]models.Book, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "bookworm-back/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google books status: %d", resp.StatusCode)
	}

	var payload struct {
		Items []struct {
			VolumeInfo struct {
				Title         string   `json:"title"`
				Authors       []string `json:"authors"`
				Description   string   `json:"description"`
				Publisher     string   `json:"publisher"`
				PublishedDate string   `json:"publishedDate"`
				PageCount     int      `json:"pageCount"`
				ImageLinks    struct {
					Thumbnail string `json:"thumbnail"`
				} `json:"imageLinks"`
				IndustryIdentifiers []struct {
					Type       string `json:"type"`
					Identifier string `json:"identifier"`
				} `json:"industryIdentifiers"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	books := make([]models.Book, 0, len(payload.Items))
	for _, item := range payload.Items {
		isbn := ""
		for _, ident := range item.VolumeInfo.IndustryIdentifiers {
			if strings.Contains(ident.Type, "ISBN") {
				isbn = ident.Identifier
				break
			}
		}

		book := models.Book{
			Title:         strings.TrimSpace(item.VolumeInfo.Title),
			Author:        strings.Join(item.VolumeInfo.Authors, ", "),
			Description:   item.VolumeInfo.Description,
			CoverURL:      item.VolumeInfo.ImageLinks.Thumbnail,
			Publisher:     item.VolumeInfo.Publisher,
			PublishedYear: extractYear(item.VolumeInfo.PublishedDate),
			Pages:         item.VolumeInfo.PageCount,
			ISBN:          isbn,
		}

		if book.Title == "" || book.Author == "" {
			continue
		}
		books = append(books, book)
	}

	return books, nil
}

type OpenLibraryProvider struct {
	client *http.Client
}

func NewOpenLibraryProvider(timeout time.Duration) *OpenLibraryProvider {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &OpenLibraryProvider{client: &http.Client{Timeout: timeout}}
}

func (p *OpenLibraryProvider) SearchByISBN(ctx context.Context, isbn string) (*models.Book, error) {
	endpoint := "https://openlibrary.org/api/books?bibkeys=ISBN:" + url.QueryEscape(isbn) + "&format=json&jscmd=data"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open library status: %d", resp.StatusCode)
	}

	var payload map[string]struct {
		Title         string `json:"title"`
		PublishDate   string `json:"publish_date"`
		NumberOfPages int    `json:"number_of_pages"`
		Publishers    []struct {
			Name string `json:"name"`
		} `json:"publishers"`
		Authors []struct {
			Name string `json:"name"`
		} `json:"authors"`
		Cover struct {
			Medium string `json:"medium"`
		} `json:"cover"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	key := "ISBN:" + isbn
	data, ok := payload[key]
	if !ok {
		return nil, ErrNoResults
	}

	authors := make([]string, 0, len(data.Authors))
	for _, a := range data.Authors {
		authors = append(authors, a.Name)
	}

	publisher := ""
	if len(data.Publishers) > 0 {
		publisher = data.Publishers[0].Name
	}

	book := models.Book{
		Title:         strings.TrimSpace(data.Title),
		Author:        strings.Join(authors, ", "),
		ISBN:          isbn,
		CoverURL:      data.Cover.Medium,
		Publisher:     publisher,
		PublishedYear: extractYear(data.PublishDate),
		Pages:         data.NumberOfPages,
		CacheSource:   "open_library",
	}

	if book.Title == "" || book.Author == "" {
		return nil, ErrNoResults
	}

	return &book, nil
}

func (p *OpenLibraryProvider) SearchByQuery(ctx context.Context, query string) ([]models.Book, error) {
	endpoint := "https://openlibrary.org/search.json?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open library status: %d", resp.StatusCode)
	}

	var payload struct {
		Docs []struct {
			Title               string   `json:"title"`
			AuthorName          []string `json:"author_name"`
			ISBN                []string `json:"isbn"`
			FirstPublishYear    int      `json:"first_publish_year"`
			NumberOfPagesMedian int      `json:"number_of_pages_median"`
			Publisher           []string `json:"publisher"`
			CoverI              int      `json:"cover_i"`
		} `json:"docs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	books := make([]models.Book, 0, len(payload.Docs))
	for _, doc := range payload.Docs {
		if strings.TrimSpace(doc.Title) == "" || len(doc.AuthorName) == 0 {
			continue
		}
		isbn := ""
		if len(doc.ISBN) > 0 {
			isbn = doc.ISBN[0]
		}
		publisher := ""
		if len(doc.Publisher) > 0 {
			publisher = doc.Publisher[0]
		}
		cover := ""
		if doc.CoverI > 0 {
			cover = "https://covers.openlibrary.org/b/id/" + strconv.Itoa(doc.CoverI) + "-M.jpg"
		}

		books = append(books, models.Book{
			Title:         doc.Title,
			Author:        strings.Join(doc.AuthorName, ", "),
			ISBN:          isbn,
			Publisher:     publisher,
			PublishedYear: doc.FirstPublishYear,
			Pages:         doc.NumberOfPagesMedian,
			CoverURL:      cover,
			CacheSource:   "open_library",
		})
	}

	if len(books) == 0 {
		return nil, ErrNoResults
	}

	if len(books) > 10 {
		books = books[:10]
	}
	return books, nil
}

func extractYear(value string) int {
	if len(value) < 4 {
		return 0
	}
	year, err := strconv.Atoi(value[:4])
	if err != nil {
		return 0
	}
	return year
}
