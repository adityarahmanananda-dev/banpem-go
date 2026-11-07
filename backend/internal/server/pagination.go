package server

import (
	"net/http"
	"strconv"
)

type Pagination struct {
	Page       int
	PerPage    int
	Total      int
	TotalPages int
	From       int
	To         int
	PrevURL    string
	NextURL    string
	HasPrev    bool
	HasNext    bool
}

func pageParams(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	switch perPage {
	case 10, 25, 50, 100:
	default:
		perPage = 25
	}
	return page, perPage
}

func pageURL(r *http.Request, page, perPage int) string {
	q := r.URL.Query()
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))
	return "?" + q.Encode()
}

func buildPagination(r *http.Request, page, perPage, total int) Pagination {
	if perPage <= 0 {
		perPage = 25
	}
	totalPages := (total + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	if page < 1 {
		page = 1
	}
	p := Pagination{Page: page, PerPage: perPage, Total: total, TotalPages: totalPages}
	if total > 0 {
		p.From = (page-1)*perPage + 1
		p.To = p.From + perPage - 1
		if p.To > total {
			p.To = total
		}
	}
	if page > 1 {
		p.HasPrev = true
		p.PrevURL = pageURL(r, page-1, perPage)
	}
	if page < totalPages {
		p.HasNext = true
		p.NextURL = pageURL(r, page+1, perPage)
	}
	return p
}

func paginateSlice[T any](items []T, page, perPage int) []T {
	if perPage <= 0 {
		perPage = 25
	}
	if page < 1 {
		page = 1
	}
	start := (page - 1) * perPage
	if start >= len(items) {
		return nil
	}
	end := start + perPage
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}
