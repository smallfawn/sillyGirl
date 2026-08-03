package core

import "testing"

func TestEncodeBucketValueKeepsIntegerFloatsAsIntegers(t *testing.T) {
	if got := encodeBucketValue(float64(8080)); got != "d:8080" {
		t.Fatalf("encodeBucketValue(float64(8080)) = %q, want d:8080", got)
	}
	if got := encodeBucketValue(1.5); got != "f:1.500000" {
		t.Fatalf("encodeBucketValue(1.5) = %q, want f:1.500000", got)
	}
}

func TestStorageEntryMatchesSearch(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  string
		search string
		want   bool
	}{
		{name: "empty search", key: "token", value: "abc", search: "", want: true},
		{name: "key substring", key: "accessToken", value: "abc", search: "token", want: true},
		{name: "value substring", key: "account", value: "OpenID-123", search: "openid", want: true},
		{name: "trimmed case insensitive", key: "NickName", value: "test", search: "  nickname ", want: true},
		{name: "not found", key: "account", value: "OpenID-123", search: "cookie", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := storageEntryMatchesSearch(tt.key, tt.value, tt.search); got != tt.want {
				t.Fatalf("storageEntryMatchesSearch(%q, %q, %q) = %v, want %v", tt.key, tt.value, tt.search, got, tt.want)
			}
		})
	}
}

func TestPaginationBounds(t *testing.T) {
	tests := []struct {
		name                 string
		page, perPage, total int
		wantPage, wantSize   int
		wantStart, wantEnd   int
	}{
		{name: "first page", page: 1, perPage: 20, total: 45, wantPage: 1, wantSize: 20, wantStart: 0, wantEnd: 20},
		{name: "last partial page", page: 3, perPage: 20, total: 45, wantPage: 3, wantSize: 20, wantStart: 40, wantEnd: 45},
		{name: "page beyond result", page: 999, perPage: 20, total: 12, wantPage: 999, wantSize: 20, wantStart: 12, wantEnd: 12},
		{name: "invalid page and size", page: 0, perPage: 0, total: 12, wantPage: 1, wantSize: 20, wantStart: 0, wantEnd: 12},
		{name: "negative page and size", page: -2, perPage: -5, total: 12, wantPage: 1, wantSize: 20, wantStart: 0, wantEnd: 12},
		{name: "size capped", page: 1, perPage: 9999, total: 500, wantPage: 1, wantSize: 200, wantStart: 0, wantEnd: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, size, start, end := paginationBounds(tt.page, tt.perPage, tt.total)
			if page != tt.wantPage || size != tt.wantSize || start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf("paginationBounds(%d, %d, %d) = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
					tt.page, tt.perPage, tt.total, page, size, start, end,
					tt.wantPage, tt.wantSize, tt.wantStart, tt.wantEnd)
			}
		})
	}
}
