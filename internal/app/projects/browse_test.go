package projects

import "testing"

func TestBuildBrowseURL(t *testing.T) {
	url, err := BuildBrowseURL(BrowseURLInput{ID: "2203306141", Name: "Side Projects"})
	if err != nil {
		t.Fatalf("BuildBrowseURL: %v", err)
	}
	want := "https://app.todoist.com/app/project/side-projects-2203306141"
	if url != want {
		t.Fatalf("unexpected url: got %q want %q", url, want)
	}
}

func TestBuildBrowseURLFallsBackSlug(t *testing.T) {
	url, err := BuildBrowseURL(BrowseURLInput{ID: "p1", Name: "   "})
	if err != nil {
		t.Fatalf("BuildBrowseURL: %v", err)
	}
	if url != "https://app.todoist.com/app/project/project-p1" {
		t.Fatalf("unexpected url: %q", url)
	}
}

func TestBuildBrowseURLRequiresID(t *testing.T) {
	if _, err := BuildBrowseURL(BrowseURLInput{Name: "Home"}); err == nil {
		t.Fatalf("expected error")
	}
}
